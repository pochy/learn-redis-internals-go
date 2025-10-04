package main

import (
	"sync" // 並行処理（複数のリクエストを同時に処理）のための排他制御（Mutex）を提供します。
)

// ====================================================================
// インメモリデータストア
// ====================================================================

// SET/GET コマンド用のデータストア: キーと値のシンプルなマップ（Goのハッシュマップ）です。
// RedisのString型を模倣しています。
var SETs = map[string]string{}

// SETsマップへの同時アクセスを防ぐためのRWMutex（読み書きロック）です。
// 読み取り（RLock）は並行して行えますが、書き込み（Lock）は排他的に行われます。
var SETsMu = sync.RWMutex{}

// HSET/HGET コマンド用のデータストア: ハッシュ名 -> キーと値のマップ、という二重構造です。
// RedisのHash型を模倣しています。
var HSETs = map[string]map[string]string{}

// HSETsマップへの同時アクセスを防ぐためのRWMutexです。
var HSETsMu = sync.RWMutex{}

// ====================================================================
// コマンドハンドラーの定義
// ====================================================================

// Handlers マップ: コマンド名（大文字の文字列）を、対応する処理関数にマッピングします。
// 例: "PING" -> ping 関数
var Handlers = map[string]func([]Value) Value{
	"PING": ping,
	"SET":  set,
	"GET":  get,
	"HSET": hset,
	"HGET": hget,
	// "HGETALL" は記事で定義されていませんが、マップには含められています。
	// "HGETALL": hgetall,
}

// ------------------------------
// PING コマンド
// ------------------------------

// ping コマンドの処理関数です。引数（args）の有無によって応答を変えます。
func ping(args []Value) Value {
	// 引数が提供されていない場合 (例: PING)
	if len(args) == 0 {
		// Simple String の "PONG" を返します。
		return Value{typ: "string", str: "PONG"}
	}
	// 引数が提供された場合 (例: PING hello)
	// 最初の引数（args[0]）の値を Simple String としてそのまま返します。
	return Value{typ: "string", str: args[0].bulk}
}

// ------------------------------
// SET コマンド
// ------------------------------

// set コマンドの処理関数です。キーと値をデータストアに保存します。
func set(args []Value) Value {
	// 引数の数（キーと値の2つ）が正しいか検証します。
	if len(args) != 2 {
		// 間違っている場合、RESP Errorを返します。
		return Value{typ: "error", str: "ERR wrong number of arguments for 'set' command"}
	}

	key := args[0].bulk   // 最初の引数をキーとして取得
	value := args[1].bulk // 2番目の引数を値として取得

	// 書き込み操作なので、排他制御のためにロックを取得します。
	SETsMu.Lock()
	// SETsマップにキーと値を保存します。
	SETs[key] = value
	// 処理が完了したらロックを解放します。
	SETsMu.Unlock()

	// 成功応答として Simple String の "OK" を返します。
	return Value{typ: "string", str: "OK"}
}

// ------------------------------
// GET コマンド
// ------------------------------

// get コマンドの処理関数です。指定されたキーの値を取得します。
func get(args []Value) Value {
	// 引数の数（キーの1つ）が正しいか検証します。
	if len(args) != 1 {
		return Value{typ: "error", str: "ERR wrong number of arguments for 'get' command"}
	}

	key := args[0].bulk // キーを取得

	// 読み取り操作なので、読み取りロックを取得します。
	SETsMu.RLock()
	// マップから値を取得します。値と、キーが存在したかどうかのフラグ（ok）を受け取ります。
	value, ok := SETs[key]
	// 処理が完了したら読み取りロックを解放します。
	SETsMu.RUnlock()

	// キーが存在しなかった場合
	if !ok {
		// Null Bulk String 応答を返します（Redisの標準的な「値がない」という応答）。
		return Value{typ: "null"}
	}

	// 値が存在した場合、その値を Bulk String として返します。
	return Value{typ: "bulk", bulk: value}
}

// ------------------------------
// HSET コマンド
// ------------------------------

// hset コマンドの処理関数です。指定されたハッシュ（外側のキー）に、フィールド（内側のキー）と値を保存します。
func hset(args []Value) Value {
	// 引数の数（ハッシュ名、キー、値の3つ）が正しいか検証します。
	if len(args) != 3 {
		return Value{typ: "error", str: "ERR wrong number of arguments for 'hset' command"}
	}

	hash := args[0].bulk  // ハッシュ名（外側のキー）
	key := args[1].bulk   // フィールドキー（内側のキー）
	value := args[2].bulk // 値

	// 書き込み操作のためロックを取得します。
	HSETsMu.Lock()
	// ハッシュ名がまだ存在しない場合、新しい内部マップ（map[string]string{}）を作成します。
	if _, ok := HSETs[hash]; !ok {
		HSETs[hash] = map[string]string{}
	}
	// 指定されたハッシュの内部マップにキーと値を保存します。
	HSETs[hash][key] = value
	HSETsMu.Unlock()

	// 成功応答として Simple String の "OK" を返します。
	return Value{typ: "string", str: "OK"}
}

// ------------------------------
// HGET コマンド
// ------------------------------

// hget コマンドの処理関数です。指定されたハッシュからフィールドの値を取得します。
func hget(args []Value) Value {
	// 引数の数（ハッシュ名、キーの2つ）が正しいか検証します。
	if len(args) != 2 {
		return Value{typ: "error", str: "ERR wrong number of arguments for 'hget' command"}
	}

	hash := args[0].bulk // ハッシュ名
	key := args[1].bulk  // フィールドキー

	// 読み取り操作のため読み取りロックを取得します。
	HSETsMu.RLock()
	// 指定されたハッシュの内部マップから値を取得します。
	value, ok := HSETs[hash][key]
	HSETsMu.RUnlock()

	// キーが存在しなかった場合（ハッシュ自体が存在しない場合も含む）
	if !ok {
		// Null Bulk String 応答を返します。
		return Value{typ: "null"}
	}

	// 値が存在した場合、その値を Bulk String として返します。
	return Value{typ: "bulk", bulk: value}
}

// ------------------------------
// HGETALL コマンド (未実装だがマップに登録されている)
// ------------------------------
// func hgetall(args []Value) Value {
//     // HGETALLの処理ロジックは記事に記述されていません。
//     // 実際には、指定されたハッシュのすべてのキーと値をRESP Arrayとして返す必要があります。
//     return Value{typ: "error", str: "ERR HGETALL is not implemented yet"}
// }
// ※上記の hgetall の実装はコメントアウトされており、エラー応答を返すなどの処理が必要です。
