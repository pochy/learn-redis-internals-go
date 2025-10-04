package main // プログラムの実行を開始するメインパッケージを宣言します。

import (
	"fmt"     // フォーマットされたI/O（主にメッセージ出力）を行うためのパッケージです。
	"net"     // ネットワークI/O（TCP通信など）を扱うためのパッケージです。
	"strings" // 文字列操作（コマンド名を大文字に変換するなど）のためのパッケージです。
)

// main関数は、プログラムが実行されたときに最初に呼び出される特別な関数です。
func main() {
	// サーバーが待ち受けを開始することをコンソールに出力します。
	fmt.Println("Listening on port :6379")

	// ----------------------------------------------------
	// 1. サーバーソケット（待ち受け口）の作成
	// ----------------------------------------------------

	// net.Listenを使って、TCPプロトコルで ":6379"（全てのネットワークインターフェースの6379番ポート）で
	// 新しい接続を待ち受けるリスナー（待ち受けソケット）を作成します。
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		// リスナーの作成に失敗した場合（例: ポートが既に使用されている）は、エラーを出力してプログラムを終了します。
		fmt.Println(err)
		return
	}

	// ----------------------------------------------------
	// 2. クライアントからの接続を待つ
	// ----------------------------------------------------

	// l.Accept() は、新しいクライアント接続が来るまで処理をブロック（停止）します。
	// 接続が確立されると、その接続を表す `conn`（net.Connインターフェース）が返されます。
	conn, err := l.Accept()
	if err != nil {
		// 接続の受け入れ中にエラーが発生した場合、エラーを出力してプログラムを終了します。
		fmt.Println(err)
		return
	}

	// defer conn.Close()
	// defer は、この関数 (main) の処理が終了する直前に conn.Close() を実行するように予約します。
	// これにより、プログラムが正常終了してもエラーで終了しても、必ず接続が閉じられることが保証されます。
	defer conn.Close()

	// ----------------------------------------------------
	// 3. 通信ループ：リクエストの処理と応答
	// ----------------------------------------------------

	// クライアントとの接続が確立された後、データを継続的に処理するための無限ループに入ります。
	for {
		// --- リクエストの読み取りとパース ---

		// 接続 (conn) を使って新しい RESP パーサー（リーダー）を作成します。
		resp := NewResp(conn)

		// クライアントから送られてきたRESP形式のデータを読み取り、Value構造体にパースします。
		value, err := resp.Read()
		if err != nil {
			// データ読み取り中にエラーが発生した場合（クライアント切断など）は、ループを終了します。
			fmt.Println(err)
			return
		}

		// --- リクエストの検証 ---

		// Redisコマンドは必ずRESP Array（配列）である必要があります。
		if value.typ != "array" {
			fmt.Println("Invalid request, expected array")
			// 処理をスキップして次のリクエストを待ちます。
			continue
		}

		// 配列が空であってはなりません（最低でもコマンド名が必要です）。
		if len(value.array) == 0 {
			fmt.Println("Invalid request, expected array length > 0")
			continue
		}

		// --- コマンド名と引数の抽出 ---

		// 配列の最初の要素がコマンド名です。それを大文字に変換します（Redisはコマンド名で大文字小文字を区別しません）。
		// .bulk を使うのは、コマンド名が常に Bulk String（例: $3\r\nSET\r\n）として送られてくるためです。
		command := strings.ToUpper(value.array[0].bulk)

		// 配列の2番目以降の要素すべてを引数（args）としてスライスします。
		args := value.array[1:]

		// --- コマンドの実行と応答 ---

		// 接続 (conn) を使って新しい RESP Writer（書き出し側）を作成します。
		writer := NewWriter(conn)

		// Handlersマップから、コマンド名に対応するハンドラー関数を検索します。
		handler, ok := Handlers[command]
		if !ok {
			// コマンドが見つからなかった場合
			fmt.Println("Invalid command: ", command)
			// エラー応答をクライアントに返します。
			writer.Write(Value{typ: "error", str: fmt.Sprintf("ERR unknown command '%s'", command)})
			continue
		}

		// ハンドラー関数を実行し、引数（args）を渡して、結果（RESP Value）を受け取ります。
		result := handler(args)

		// 実行結果（Value）を Writer.Write() で RESP バイト列に変換し、クライアントに送信します。
		writer.Write(result)
	}
}
