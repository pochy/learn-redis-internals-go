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

// RESPのパーサー全体を管理するための構造体です。
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
