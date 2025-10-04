package main // プログラムは「main」パッケージから実行されます。

import (
	"bufio"   // バッファリングされたI/O（入出力）を提供します。効率的な読み取りのために使われます。
	"fmt"     // フォーマットされたI/O、主にデバッグやエラーメッセージの出力に使われます。
	"io"      // I/Oプリミティブ（基本的な入出力操作）を提供します。`io.Reader`などで使います。
	"strconv" // 文字列と基本的なデータ型（数値など）の間で変換を行います。
)

// RESPプロトコルで使用される型を示す定数です。
// 各定数は、データ型の先頭のバイト（文字）に対応しています。
const (
	STRING  = '+' // Simple Strings（シンプルな文字列）のプレフィックス
	ERROR   = '-' // Errors（エラー）のプレフィックス
	INTEGER = ':' // Integers（整数）のプレフィックス
	BULK    = '$' // Bulk Strings（バルク文字列、長さ情報を持つ文字列）のプレフィックス
	ARRAY   = '*' // Arrays（配列）のプレフィックス
)

// RESPでやり取りされるデータを格納するための構造体（struct）です。
// RESPは様々なデータ型を持つため、それらをまとめて保持できるようにしています。
type Value struct {
	typ   string  // データの種類（例: "array", "bulk" など）
	str   string  // Simple StringやErrorなどの文字列データ用
	num   int     // Integerなどの数値データ用
	bulk  string  // Bulk Stringなどのバルクデータ用
	array []Value // Array型の場合、要素（Valueの配列）を格納
}

// RESPのパーサー全体を管理するための構造体です。（リーダー側）
type Resp struct {
	reader *bufio.Reader // データを効率的に読み取るためのバッファリングされたリーダー
}

// Resp構造体の新しいインスタンスを作成し、初期化するコンストラクタ関数です。
// rd (io.Reader) は、データが流れてくる元（例: ネットワーク接続）です。
func NewResp(rd io.Reader) *Resp {
	// io.Readerを bufio.NewReader でラップ（包んで）し、バッファリングされたリーダーを作成します。
	return &Resp{reader: bufio.NewReader(rd)}
}

// 一行全体を読み取るためのメソッド（Respに紐づいた関数）です。
// RESPプロトコルでは、データは通常 '\r\n' (CRLF) で区切られています。
func (r *Resp) readLine() (line []byte, n int, err error) {
	// 無限ループでバイトを一つずつ読み込みます。
	for {
		b, err := r.reader.ReadByte() // リーダーから次の1バイトを読み込みます。
		if err != nil {
			return nil, 0, err // エラーがあれば即座に返します。
		}
		n += 1                 // 読み込んだバイト数をカウントします。
		line = append(line, b) // 読み込んだバイトを行の末尾に追加します。
		// 行の末尾2バイトが '\r' (CR) であれば、CRLF（'\r\n'）の読み込みが完了したと判断し、ループを抜けます。
		// 注意: このコードは '\r' をチェックしていますが、実際には '\r\n' をチェックすべきです。
		// この実装では、'\r' の次に ReadByte で '\n' が読み込まれることを期待しています（ただし、この関数内では '\r' しかチェックしていません）。
		// 正確には行の末尾が '\r\n' であるかをチェックする必要がありますが、このコードのロジックに従います。
		if len(line) >= 2 && line[len(line)-2] == '\r' {
			break
		}
	}
	// 行全体から末尾の '\r\n' (CRLF) を除くために、最後の2バイト（'\r'と'\n'）を除去して返します。
	return line[:len(line)-2], n, nil
}

// RESPプロトコルから整数値を読み取るためのメソッドです。
// 整数は通常、':' の後に続き、CRLF で終わります。
func (r *Resp) readInteger() (x int, n int, err error) {
	// まず、整数の文字列表現がある行全体を読み取ります。
	line, n, err := r.readLine()
	if err != nil {
		return 0, 0, err
	}

	// 読み取ったバイトスライス (line) を文字列に変換し、strconv.ParseInt で64ビット整数に変換します。
	i64, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		// 変換に失敗した場合（例: 数字以外の文字が含まれていた場合）
		return 0, n, err
	}
	// 64ビット整数を int 型に変換して返します。
	return int(i64), n, nil
}

// RESPデータの最初のバイトを読み取り、対応する型に応じてパース（解析）関数を呼び出します。
// これがデータの読み取りを開始するメインのエントリポイントです。
func (r *Resp) Read() (Value, error) {
	// データの型を示す最初の1バイトを読み取ります（例: '*'、'$' など）。
	_type, err := r.reader.ReadByte()
	if err != nil {
		// データの終わり（EOF）などのエラーがあれば返します。
		return Value{}, err
	}

	// 読み取った型に応じて、適切なパース関数を呼び出します。
	switch _type {
	case ARRAY:
		return r.readArray() // '*' の場合、配列のパース関数を呼び出します。
	case BULK:
		return r.readBulk() // '$' の場合、バルク文字列のパース関数を呼び出します。
	default:
		// 未知の型が来た場合は、エラーメッセージを出力し、空のValueを返します。
		fmt.Printf("Unknown type: %v", string(_type))
		return Value{}, nil
	}
}

// RESP配列（Array）を読み取るためのメソッドです。
// 配列は '*' の後に要素数、そして各要素のデータが続きます。
func (r *Resp) readArray() (Value, error) {
	v := Value{}
	v.typ = "array" // Valueの型を "array" に設定します。

	// 配列の要素数（長さ）を読み取ります。これは readInteger でパースされます。
	len, _, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// 配列の要素を格納するためのスライスを、容量0で作成します。
	v.array = make([]Value, 0)
	// 要素数分だけループし、各要素を再帰的に読み取ります。
	for i := 0; i < len; i++ {
		// 配列の各要素は、再び Read() メソッドでパースされます（再帰的な処理）。
		val, err := r.Read()
		if err != nil {
			return v, err
		}

		// パースされた要素を配列に追加します。
		v.array = append(v.array, val)
	}

	return v, nil
}

// RESPバルク文字列（Bulk String）を読み取るためのメソッドです。
// バルク文字列は '$' の後に長さ、CRLF、データ本体、CRLF が続きます。
func (r *Resp) readBulk() (Value, error) {
	v := Value{}
	v.typ = "bulk" // Valueの型を "bulk" に設定します。

	// バルク文字列のデータの長さ（バイト数）を読み取ります。
	len, _, err := r.readInteger()
	if err != nil {
		return v, err
	}

	// 読み込む長さ分のバイトスライスを作成します。
	bulk := make([]byte, len)

	// リーダーから、指定された長さ（len）のデータを直接読み込みます。
	// この読み込みで、データ本体（文字列）が bulk スライスに格納されます。
	r.reader.Read(bulk)

	// バイトスライスを文字列に変換し、Valueに格納します。
	v.bulk = string(bulk)

	// Bulk String のデータ本体の後には、必ず末尾の CRLF が続きます。
	// この readLine() は、その CRLF を読み捨て（スキップ）するために呼び出されます。
	r.readLine()

	return v, nil
}

// ====================================================================
// RESPシリアライザー (Marshal/Writer)
// ====================================================================

// Value構造体をRESP形式のバイト列（[]byte）に変換（シリアライズ）するメインメソッドです。
func (v Value) Marshal() []byte {
	// Valueの型に応じて、適切なマーシャリング関数を呼び出します。
	switch v.typ {
	case "array":
		return v.marshalArray()
	case "bulk":
		return v.marshalBulk()
	case "string":
		return v.marshalString()
	case "null":
		return v.marshallNull()
	case "error":
		return v.marshallError()
	default:
		// 未知の型の場合は空のバイト列を返します。
		return []byte{}
	}
}

// Simple String（+）をRESP形式に変換します。
// 形式: +データ本体\r\n
func (v Value) marshalString() []byte {
	var bytes []byte
	// 1. プレフィックス '+' を追加
	bytes = append(bytes, STRING)
	// 2. 文字列データ本体を追加
	bytes = append(bytes, v.str...)
	// 3. 終端の CRLF (\r\n) を追加
	bytes = append(bytes, '\r', '\n')

	return bytes
}

// Bulk String（$）をRESP形式に変換します。
// 形式: $バイト数\r\nデータ本体\r\n
func (v Value) marshalBulk() []byte {
	var bytes []byte
	// 1. プレフィックス '$' を追加
	bytes = append(bytes, BULK)
	// 2. データ本体のバイト数を文字列に変換して追加（例: 5）
	bytes = append(bytes, strconv.Itoa(len(v.bulk))...)
	// 3. バイト数ヘッダーの終端 CRLF (\r\n) を追加
	bytes = append(bytes, '\r', '\n')
	// 4. 文字列データ本体を追加
	bytes = append(bytes, v.bulk...)
	// 5. データ本体の終端 CRLF (\r\n) を追加
	bytes = append(bytes, '\r', '\n')

	return bytes
}

// Array（*）をRESP形式に変換します。
// 形式: *要素数\r\n[要素1のRESP表現][要素2のRESP表現]...
func (v Value) marshalArray() []byte {
	len := len(v.array) // 配列の要素数を取得
	var bytes []byte
	// 1. プレフィックス '*' を追加
	bytes = append(bytes, ARRAY)
	// 2. 要素数を文字列に変換して追加（例: 3）
	bytes = append(bytes, strconv.Itoa(len)...)
	// 3. 要素数ヘッダーの終端 CRLF (\r\n) を追加
	bytes = append(bytes, '\r', '\n')

	// 4. 配列の各要素をループ処理
	for i := 0; i < len; i++ {
		// 各要素（Value）に対して再帰的に Marshal() を呼び出し、RESPバイト列を取得
		// その結果を全体のバイト列に追加します。
		bytes = append(bytes, v.array[i].Marshal()...)
	}

	return bytes
}

// Error（-）をRESP形式に変換します。
// 形式: -エラーメッセージ\r\n
func (v Value) marshallError() []byte {
	var bytes []byte
	// 1. プレフィックス '-' を追加
	bytes = append(bytes, ERROR)
	// 2. エラーメッセージの文字列を追加
	bytes = append(bytes, v.str...)
	// 3. 終端の CRLF (\r\n) を追加
	bytes = append(bytes, '\r', '\n')

	return bytes
}

// Null Bulk String をRESP形式に変換します。
// 形式: $-1\r\n (Nullは固定の表現です)
func (v Value) marshallNull() []byte {
	// Nullは常にこの5バイトの固定値です。
	return []byte("$-1\r\n")
}

// ====================================================================
// Writer 構造体とメソッド
// ====================================================================

// RESP応答をネットワーク接続に書き込むための構造体です。
type Writer struct {
	writer io.Writer // 実際にデータを書き込むターゲット（例: net.Conn）
}

// Writer構造体の新しいインスタンスを作成するコンストラクタ関数です。
// w (io.Writer) は、書き出し先のネットワーク接続などです。
func NewWriter(w io.Writer) *Writer {
	// io.Writer を保持する Writer オブジェクトを返します。
	return &Writer{writer: w}
}

// Value構造体をRESPバイト列に変換し、io.Writerを通じて書き込みます。
func (w *Writer) Write(v Value) error {
	// 1. ValueオブジェクトをRESP形式のバイト列に変換します。
	var bytes = v.Marshal()

	// 2. io.Writer の Write メソッドを使って、変換したバイト列をネットワークなどに書き込みます。
	_, err := w.writer.Write(bytes)
	if err != nil {
		// 書き込みエラーが発生した場合はそれを返します。
		return err
	}

	// 正常に書き込みが完了しました。
	return nil
}
