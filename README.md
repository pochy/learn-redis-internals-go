# Redis 互換 TCP サーバー - Go 言語実装

## プロジェクト概要

この **Golang** のプログラムは、非常にシンプルな **TCP サーバー** として機能し、**Redis** の通信プロトコルである **RESP (REdis Serialization Protocol)** を使ってクライアントからの命令を受け取り、固定の応答を返す一連の流れを実装しています。

> **参考資料**: このプロジェクトは [Build Redis from scratch](https://www.build-redis-from-scratch.dev/en/introduction) のチュートリアルを参考に作成されています。Redis の内部動作を理解し、データベースの低レベルな詳細を学習することを目的としています。

### 学習目標

- TCP サーバーの基本的な仕組みを理解する
- RESP プロトコルの解析方法を学ぶ
- Go 言語でのネットワークプログラミングの基礎を習得する
- Redis クライアントとの通信の仕組みを理解する

### 前提知識

- Go 言語の基本的な構文（構造体、メソッド、エラーハンドリング）
- ネットワークの基本的な概念（TCP、ポート、ソケット）
- Redis の基本的な使い方（redis-cli コマンド）

---

## 目次

1. [プロジェクト構成](#プロジェクト構成)
2. [サーバーの起動と待ち受け](#1-サーバーの起動と待ち受けmain-関数開始)
3. [クライアント接続の受け入れ](#2-クライアント接続の受け入れ)
4. [データの読み書きループ](#3-データの読み書きループサーバーの主な処理)
5. [RESP プロトコルパーサーの詳細](#resp-redis-serialization-protocol-パーサーの詳細解説)
6. [RESP Writer（レスポンス送信）の詳細解説](#resp-writerレスポンス送信の詳細解説)
7. [Redis コマンドハンドラーの実装](#redis-コマンドハンドラーの実装)
8. [実行方法とテスト](#実行方法とテスト)
9. [トラブルシューティング](#トラブルシューティング)

---

## プロジェクト構成

```
build-your-own-redis-go/
├── main.go          # TCPサーバーのメインロジックとコマンド処理
├── resp.go          # RESPプロトコルパーサーとWriter
├── handler.go       # Redisコマンドハンドラー（PING、SET、GET、HSET、HGET）
└── README.md        # このファイル
```

### ファイルの役割

- **main.go**: TCP サーバーの起動、クライアント接続の受け入れ、コマンド処理ループ
- **resp.go**: RESP プロトコルで送信されるデータの解析（パース）とシリアライズ機能
- **handler.go**: Redis コマンドの実装（PING、SET、GET、HSET、HGET）とデータストア管理
- **README.md**: プロジェクトの詳細な説明と学習ガイド

---

## 1. サーバーの起動と待ち受け（`main` 関数開始）

---

## 1\. サーバーの起動と待ち受け（`main` 関数開始）

プログラムの実行は `main` 関数から始まります。ここでは、サーバーがどのように起動し、クライアントからの接続を待つ準備をするかを詳しく見ていきます。

### 1.1 メッセージの出力

```go
fmt.Println("Listening on port :6379")
```

**何をしているか:**

- コンソールに「ポート **6379** で待ち受けを開始する」というメッセージを出力
- サーバーが起動中であることをユーザーに知らせる
- デバッグやログの目的でも使用される

**なぜ 6379 番ポートなのか:**

- Redis の標準ポート番号
- 他の Redis クライアント（redis-cli など）がデフォルトで接続を試みるポート
- ポート番号は 0-65535 の範囲で、1024 以下はシステム用に予約されている

### 1.2 TCP リスナーの作成

```go
l, err := net.Listen("tcp", ":6379")
```

**`net.Listen`関数の詳細:**

- **第 1 引数 "tcp"**: 通信プロトコルとして **TCP (Transmission Control Protocol)** を指定
  - TCP は信頼性の高い通信プロトコル（データの順序保証、再送機能など）
  - HTTP、HTTPS、Redis など多くのアプリケーションで使用
- **第 2 引数 ":6379"**: サーバーが接続を待つアドレスとポートを指定
  - `:` の前が空 = すべてのネットワークインターフェース（外部からの接続も受け入れ）
  - `6379` = Redis のデフォルトポート番号

**戻り値:**

- `l` (リスナー): 成功時に返される `net.Listener` インターフェース
- `err` (エラー): 失敗時に返されるエラー情報

**実際の動作:**

1. オペレーティングシステムに「6379 番ポートで接続を待ち受けてください」と依頼
2. OS がポートをバインド（占有）し、接続要求を待機
3. 成功するとリスナーオブジェクトが返される

### 1.3 エラー処理

```go
if err != nil {
    fmt.Println(err)
    return
}
```

**エラーが発生する可能性のあるケース:**

- **ポートが既に使用中**: 他のプログラム（本物の Redis サーバーなど）が 6379 番ポートを使用している
- **権限不足**: 1024 番以下のポートを使用する場合に管理者権限が必要
- **ネットワーク設定の問題**: ファイアウォールやネットワーク設定による制限

**エラーハンドリングの重要性:**

- プログラムが予期しない動作を防ぐ
- ユーザーに問題の原因を伝える
- リソースの適切な解放を保証する

---

## 2. クライアント接続の受け入れ

サーバーは、リスナーが作成されると、クライアントからの接続を待ちます。ここでは、クライアントが接続してきた時の処理を詳しく見ていきます。

### 2.1 接続のブロックと待機

```go
conn, err := l.Accept()
```

**`l.Accept()`の動作:**

- 新しいクライアントがサーバーに接続してくるまで、プログラムの実行を **一時停止（ブロック）** します
- これは**同期処理**で、接続が来るまで他の処理は実行されません
- クライアント（`redis-cli`や別のプログラム）が接続を試みると、接続が確立されます

**戻り値:**

- `conn` (コネクション): `net.Conn`インターフェースを実装したオブジェクト
  - クライアントとの間で実際にデータをやり取りするための「**通信路**」
  - `Read()`、`Write()`、`Close()`などのメソッドを持つ
- `err` (エラー): 接続受け入れ中に発生したエラー

**接続の確立プロセス:**

1. クライアントがサーバーの IP アドレスとポート（6379）に接続要求を送信
2. OS のネットワークスタックが接続要求を受け取り
3. `Accept()`が接続を確立し、通信路を作成
4. サーバーとクライアント間でデータの送受信が可能になる

### 2.2 エラー処理

```go
if err != nil {
    fmt.Println(err)
    return
}
```

**接続受け入れでエラーが発生するケース:**

- **ネットワークエラー**: ネットワークの障害や設定の問題
- **リソース不足**: システムのメモリやファイルディスクリプタの不足
- **セキュリティ制限**: ファイアウォールやセキュリティポリシーによる制限
- **サーバーの停止**: サーバープロセスが終了している

### 2.3 接続のクリーンアップ予約

```go
defer conn.Close()
```

**`defer`キーワードの詳細:**

- `defer`は、この`main`関数が **終了する直前** に、指定された処理を必ず実行するように予約します
- 関数の終了方法に関係なく実行される（正常終了、エラー終了、panic など）
- **LIFO（Last In, First Out）**の順序で実行される

**なぜ`defer`を使うのか:**

- **リソースリークの防止**: 接続を適切に閉じることで、システムリソースを解放
- **確実なクリーンアップ**: プログラムが予期しない終了をしても接続が閉じられる
- **コードの簡潔性**: エラー処理の各箇所で`Close()`を書く必要がない

**実際の動作:**

1. `defer conn.Close()`が実行され、関数終了時の処理として登録される
2. `main`関数が終了する直前（`return`の実行時）に`conn.Close()`が呼び出される
3. TCP 接続が適切に閉じられ、リソースが解放される

---

## 3. データの読み書きループ（サーバーの主な処理）

接続が確立されたら、`for {}` の **無限ループ** に入り、クライアントとのデータのやり取りを継続します。これがサーバーの核となる処理です。

### 3.1 RESP パーサーの初期化

```go
resp := NewResp(conn)
```

**`NewResp`関数の役割:**

- クライアントとの通信路 `conn` を使って、RESP プロトコル形式のデータを解析するための `Resp` オブジェクトを新しく作成
- `bufio.Reader`でラップすることで、効率的なデータ読み取りを実現
- ループの各反復で新しいパーサーインスタンスを作成（状態のリセット）

**なぜループ内で毎回作成するのか:**

- パーサーの状態をリセットして、前回の処理の影響を排除
- 各リクエストを独立して処理するため
- エラーが発生した場合の影響を最小限に抑える

### 3.2 クライアントデータの読み取りと解析

```go
value, err := resp.Read()
```

**`resp.Read()`の動作:**

- クライアントから送信された **RESP 形式のデータ**（例：`*2\r\n$4\r\nPING\r\n$4\r\nTEST\r\n`）を読み取り
- バイナリデータを Go の `Value` 構造体に変換（パース）
- この処理は、クライアントがデータを送信するまでブロックされる

**RESP データの例:**

```
*2\r\n$4\r\nPING\r\n$4\r\nTEST\r\n
```

- `*2`: 2 つの要素を持つ配列
- `$4\r\nPING\r\n`: 4 文字のバルク文字列 "PING"
- `$4\r\nTEST\r\n`: 4 文字のバルク文字列 "TEST"

**パース結果:**

```go
Value{
    typ: "array",
    array: [
        Value{typ: "bulk", bulk: "PING"},
        Value{typ: "bulk", bulk: "TEST"}
    ]
}
```

### 3.3 読み取りエラー処理

```go
if err != nil {
    fmt.Println(err)
    return
}
```

**エラーが発生する主なケース:**

- **`io.EOF`**: クライアントが接続を閉じた（最も一般的）
- **ネットワークエラー**: 接続の切断やタイムアウト
- **不正なデータ形式**: RESP プロトコルに準拠しないデータ
- **リソース不足**: メモリ不足など

**エラーハンドリングの戦略:**

- エラーをログに出力して問題を特定
- `return`でプログラムを終了（単一接続サーバーのため）
- `defer conn.Close()`により接続のクリーンアップが保証される

### 3.4 受信データの出力（デバッグ用）

```go
fmt.Println(value)
```

**デバッグ出力の目的:**

- クライアントから受信したデータが正しくパースされているかを確認
- 開発時の動作確認とトラブルシューティング
- RESP プロトコルの学習と理解の促進

**出力例:**

```
{array [{bulk PING} {bulk TEST}]}
```

### 3.5 固定応答の送信

```go
conn.Write([]byte("+OK\r\n"))
```

**応答の詳細:**

- 受信したデータの内容に関係なく、サーバーはクライアントに対して **"+OK\r\n"** という **固定の応答** を送り返します
- これは RESP の **Simple String** 形式の応答
- `+` は Simple String のプレフィックス
- `OK` は成功を表すメッセージ
- `\r\n` は RESP プロトコルで必須の改行コード

**`conn.Write()`の動作:**

- バイトデータ (`[]byte`) を使って応答をクライアントに送信
- ネットワーク経由でクライアントにデータが届く
- 書き込みが完了するまでブロックされる

**実際の通信フロー:**

1. クライアント: `*2\r\n$4\r\nPING\r\n$4\r\nTEST\r\n` を送信
2. サーバー: データを受信・パース
3. サーバー: `+OK\r\n` を送信
4. クライアント: 応答を受信
5. ループが継続し、次のリクエストを待機

---

## 4. プログラムの終了

無限ループが終了条件（接続切断エラーなど）を満たした場合、`main` 関数が終了します。

### 4.1 終了プロセス

1. **エラー発生時**: `resp.Read()`でエラーが発生し、`return`が実行される
2. **`defer`の実行**: `defer conn.Close()`が実行され、クライアントとの接続が正式に閉じられる
3. **リソース解放**: TCP 接続、ファイルディスクリプタなどのシステムリソースが解放される
4. **プログラム終了**: `main`関数が終了し、プログラムが完全に終了する

### 4.2 このサーバーの特徴

このプログラムは、クライアントとの接続を一つだけ処理する、**最も基本的な単一接続のサーバー** の例です。

**制限事項:**

- 同時に 1 つのクライアント接続しか処理できない
- クライアントが切断すると、サーバーも終了する
- 実際の Redis サーバーとは異なり、データの永続化や複雑なコマンド処理は行わない

**実際のサーバーとの違い:**

- 実際のサーバーは、通常、複数のクライアント接続を同時に処理するために、**並行処理（ゴルーチン）** を使います
- 本格的なサーバーでは、データベース機能、認証、ログ機能などが追加されます

---

## RESP (REdis Serialization Protocol) パーサーの詳細解説

**RESP (REdis Serialization Protocol) パーサー** のコア部分である `Resp` 構造体のメソッド（関数）について、データの読み取りから解析までの流れを詳細に解説します。

このパーサーは、ネットワーク接続から流れてくるバイトデータを、意味のある **Value** 構造体（RESP データ型）に変換する役割を担っています。

---

### 5.1 パーサーの準備と基本的な読み取り

パーサーがクライアントとの通信路からデータを取得する、最も基本的な処理です。

#### 5.1.1 初期化 (`NewResp`)

```go
func NewResp(rd io.Reader) *Resp {
    return &Resp{reader: bufio.NewReader(rd)}
}
```

**この関数の役割:**

- パーサーを作成するコンストラクタ関数
- ネットワーク接続 (`io.Reader`) を受け取り、それを **`bufio.Reader`** でラップ（包んで）します

**`bufio.Reader`の利点:**

- **バッファリング** を行うことで、ネットワークからバイトを一つずつ読み取る際の効率を大幅に向上
- システムコールの回数を減らし、パフォーマンスを向上
- 内部バッファにデータを蓄積し、必要に応じて提供

**実際の動作:**

1. `io.Reader`インターフェースを受け取る（`net.Conn`など）
2. `bufio.NewReader()`でバッファリングされたリーダーを作成
3. `Resp`構造体のインスタンスを作成して返す

#### 5.1.2 行末までの読み取り (`readLine`)

```go
func (r *Resp) readLine() (line []byte, n int, err error) { /* ... */ }
```

**RESP プロトコルの改行規則:**

- ほとんどのデータ型（文字列の長さ、整数の値など）は、データ本体の後に **CRLF**（`\r\n`、キャリッジリターンとラインフィード）という改行コードが続く
- これは HTTP プロトコルと同じ改行コード形式

**`readLine`メソッドの動作:**

1. `bufio.Reader` の `ReadByte()` を使ってバイトを一つずつ読み込み
2. 読み込んだバイトを `line` スライスに追加
3. 行の末尾が `\r\n` であることを検出すると、読み込みを終了
4. 最終的に、**`\r\n` を除いた** データ本体のバイトスライス（`line[:len(line)-2]`）を返す

**戻り値:**

- `line`: CRLF を除いたデータ本体
- `n`: 読み込んだ総バイト数（CRLF 含む）
- `err`: エラー情報

#### 5.1.3 整数値の読み取りと変換 (`readInteger`)

```go
func (r *Resp) readInteger() (x int, n int, err error) { /* ... */ }
```

**このメソッドの用途:**

- RESP で使われる **長さ情報** や **整数型** の値を解析
- 配列の要素数、バルク文字列の長さなどに使用

**処理の流れ:**

1. `r.readLine()` を呼び出し、整数の文字列表現（例: "3" や "1024"）を取得
2. **`strconv.ParseInt`** を使用して、取得したバイトスライス（文字列）を実際の数値 (`int64`) に変換
3. その数値を `int` 型として返す

**エラーハンドリング:**

- 文字列が数値として解釈できない場合、エラーを返す
- オーバーフローやアンダーフローの場合もエラーとして処理

---

### 5.2 メインのパース処理 (`Read`)

クライアントから送られてきたデータの解析は、この `Read` メソッドから始まります。

```go
func (r *Resp) Read() (Value, error) {
    _type, err := r.reader.ReadByte()
    if err != nil {
        return Value{}, err
    }

    switch _type {
    case ARRAY:
        return r.readArray()
    case BULK:
        return r.readBulk()
    default:
        fmt.Printf("Unknown type: %v", string(_type))
        return Value{}, nil
    }
}
```

#### 5.2.1 最初のバイトの読み取り

```go
_type, err := r.reader.ReadByte()
```

**RESP プロトコルの型識別:**

- `r.reader.ReadByte()` を使って、データの型を示す **最初の 1 バイト**（**プレフィックス**）を読み取る
- RESP では、この 1 バイトがデータ型を決定する：
  - `*` = 配列（ARRAY）
  - `$` = バルク文字列（BULK）
  - `+` = シンプル文字列（STRING）
  - `-` = エラー（ERROR）
  - `:` = 整数（INTEGER）

#### 5.2.2 型の判別と処理振り分け

```go
switch _type {
case ARRAY:
    return r.readArray()
case BULK:
    return r.readBulk()
default:
    fmt.Printf("Unknown type: %v", string(_type))
    return Value{}, nil
}
```

**処理の流れ:**

1. 読み取ったプレフィックス（`_type`）に基づき、`switch` 文で適切な解析メソッドに処理を振り分け
2. `*` の場合は `readArray()` を呼び出し
3. `$` の場合は `readBulk()` を呼び出し
4. 未知の型の場合はエラーメッセージを出力

**エラーハンドリング:**

- データの終わり（EOF）などのエラーがあれば即座に返す
- 未知の型の場合は警告を出力して空の Value を返す

---

## 3\. 複合データ型の解析

### 3.1. 配列の解析 (`readArray`)

RESP の **配列** (`*` で始まるデータ) は、他のデータ型（バルク文字列など）を要素として含む、**複合的なデータ型** です。

1.  **型と要素数の読み取り**:
    - `v.typ = "array"` を設定します。
    - `r.readInteger()` を呼び出し、配列に含まれる **要素の数 (len)** を読み取ります。
2.  **要素の反復処理**:
    - 要素数 `len` の回数だけ `for` ループを回します。
3.  **再帰的な呼び出し**:
    - ループ内で、再び **`r.Read()`** メソッドを呼び出します。これが **再帰的な処理** の要点です。配列の要素は、バルク文字列かもしれないし、さらに別の配列かもしれません。`Read()` が要素のプレフィックスを読み取り、適切な解析を行います。
4.  **要素の格納**:
    - 解析された要素 (`val`) を配列のリスト (`v.array`) に追加（`append`）し、すべての要素が処理されるまでこれを繰り返します。

### 3.2. バルク文字列の解析 (`readBulk`)

RESP の **バルク文字列** (`$` で始まるデータ) は、クライアントからサーバーへ送信されるコマンド引数など、まとまったデータに使われます。

1.  **長さの読み取り**:
    - `r.readInteger()` を呼び出し、バルク文字列の **バイト数（長さ）** を読み取ります。
2.  **データ本体の読み取り**:
    - 読み取った長さ (`len`) のバイトスライス (`bulk`) を作成します。
    - `r.reader.Read(bulk)` を呼び出し、**正確にその長さ分だけ** のデータをネットワークから読み込み、`bulk` スライスに格納します。
3.  **文字列への変換**:
    - 読み込んだバイトスライスを Go の `string` 型に変換し、`v.bulk` に格納します。
4.  **CRLF の読み捨て**:
    - バルク文字列のデータ本体の **直後** には、必ず **末尾の CRLF** があります。これをスキップ（読み捨て）するために、`r.readLine()` を **戻り値を無視して** 呼び出します。

### 5.3 複合データ型の解析

#### 5.3.1 配列の解析 (`readArray`)

RESP の **配列** (`*` で始まるデータ) は、他のデータ型（バルク文字列など）を要素として含む、**複合的なデータ型** です。

```go
func (r *Resp) readArray() (Value, error) {
    v := Value{}
    v.typ = "array"

    len, _, err := r.readInteger()
    if err != nil {
        return v, err
    }

    v.array = make([]Value, 0)
    for i := 0; i < len; i++ {
        val, err := r.Read()
        if err != nil {
            return v, err
        }
        v.array = append(v.array, val)
    }

    return v, nil
}
```

**処理の詳細:**

1. **型と要素数の読み取り**:

   - `v.typ = "array"` を設定して Value の型を指定
   - `r.readInteger()` を呼び出し、配列に含まれる **要素の数 (len)** を読み取る

2. **要素の反復処理**:

   - 要素数 `len` の回数だけ `for` ループを実行
   - `make([]Value, 0)` で空のスライスを作成（容量 0 で初期化）

3. **再帰的な呼び出し**:

   - ループ内で、再び **`r.Read()`** メソッドを呼び出し
   - これが **再帰的な処理** の要点：配列の要素は、バルク文字列かもしれないし、さらに別の配列かもしれない
   - `Read()` が要素のプレフィックスを読み取り、適切な解析を行う

4. **要素の格納**:
   - 解析された要素 (`val`) を配列のリスト (`v.array`) に追加（`append`）
   - すべての要素が処理されるまでこれを繰り返す

**配列の例:**

```
*2\r\n$4\r\nPING\r\n$4\r\nTEST\r\n
```

- `*2`: 2 つの要素を持つ配列
- `$4\r\nPING\r\n`: 1 番目の要素（バルク文字列 "PING"）
- `$4\r\nTEST\r\n`: 2 番目の要素（バルク文字列 "TEST"）

#### 5.3.2 バルク文字列の解析 (`readBulk`)

RESP の **バルク文字列** (`$` で始まるデータ) は、クライアントからサーバーへ送信されるコマンド引数など、まとまったデータに使われます。

```go
func (r *Resp) readBulk() (Value, error) {
    v := Value{}
    v.typ = "bulk"

    len, _, err := r.readInteger()
    if err != nil {
        return v, err
    }

    bulk := make([]byte, len)
    r.reader.Read(bulk)
    v.bulk = string(bulk)

    r.readLine()

    return v, nil
}
```

**処理の詳細:**

1. **長さの読み取り**:

   - `r.readInteger()` を呼び出し、バルク文字列の **バイト数（長さ）** を読み取る
   - この長さは、データ本体のバイト数を表す

2. **データ本体の読み取り**:

   - 読み取った長さ (`len`) のバイトスライス (`bulk`) を作成
   - `r.reader.Read(bulk)` を呼び出し、**正確にその長さ分だけ** のデータをネットワークから読み込み
   - `bulk` スライスに格納される

3. **文字列への変換**:

   - 読み込んだバイトスライスを Go の `string` 型に変換
   - `v.bulk` に格納する

4. **CRLF の読み捨て**:
   - バルク文字列のデータ本体の **直後** には、必ず **末尾の CRLF** がある
   - これをスキップ（読み捨て）するために、`r.readLine()` を **戻り値を無視して** 呼び出す

**バルク文字列の例:**

```
$4\r\nPING\r\n
```

- `$4`: 4 バイトのバルク文字列
- `\r\n`: 長さの後の CRLF
- `PING`: データ本体（4 バイト）
- `\r\n`: データ本体の後の CRLF

### 5.4 パーサーの動作まとめ

この一連の流れにより、サーバーはネットワーク通信路から流れてくるバイナリデータを、Go の `Value` 構造体に綺麗に分解・格納することができます。

**パーサーの特徴:**

- **再帰的**: 配列の要素として他の配列やバルク文字列を含むことができる
- **型安全**: 各データ型に応じた適切な処理を行う
- **エラーハンドリング**: 不正なデータやネットワークエラーに対応
- **効率的**: バッファリングによりネットワーク I/O を最適化

---

## RESP Writer（レスポンス送信）の詳細解説

前回のパーサー（Reader）では、クライアントから送信された RESP データを`Value`構造体に変換しました。今度は、その逆の処理である**Writer**を実装します。Writer は、サーバーがクライアントにレスポンスを送信するための機能です。

### 6.1 Writer の概要

**Writer の役割:**

- `Value`構造体を RESP 形式のバイト列に変換（シリアライズ）
- 変換したバイト列をネットワーク接続に書き込み
- クライアントが理解できる形式でレスポンスを送信

**シリアライズとは:**

- データ構造（`Value`）をバイト列（`[]byte`）に変換する処理
- ネットワーク経由でデータを送信するために必要
- パーサー（デシリアライズ）の逆の処理

### 6.2 Value のシリアライザー（Marshal）

#### 6.2.1 メインの Marshal メソッド

```go
func (v Value) Marshal() []byte {
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
        return []byte{}
    }
}
```

**このメソッドの動作:**

1. `Value`の型（`v.typ`）を確認
2. 型に応じて適切なシリアライズ関数を呼び出し
3. RESP 形式のバイト列を返す

**なぜ switch 文を使うのか:**

- 各データ型ごとに異なる RESP 形式があるため
- 型安全な処理を実現
- 拡張性を保つ（新しい型を追加しやすい）

#### 6.2.2 Simple String（シンプル文字列）のシリアライズ

```go
func (v Value) marshalString() []byte {
    var bytes []byte
    bytes = append(bytes, STRING)        // '+' を追加
    bytes = append(bytes, v.str...)      // 文字列本体を追加
    bytes = append(bytes, '\r', '\n')    // CRLFを追加

    return bytes
}
```

**RESP 形式:** `+文字列\r\n`

**処理の詳細:**

1. **プレフィックス追加**: `STRING`定数（`+`）を追加
2. **データ本体追加**: `v.str`の内容をバイト列として追加
3. **終端追加**: `\r\n`（CRLF）を追加

**例:**

- 入力: `Value{typ: "string", str: "OK"}`
- 出力: `+OK\r\n`

#### 6.2.3 Bulk String（バルク文字列）のシリアライズ

```go
func (v Value) marshalBulk() []byte {
    var bytes []byte
    bytes = append(bytes, BULK)                              // '$' を追加
    bytes = append(bytes, strconv.Itoa(len(v.bulk))...)     // 長さを追加
    bytes = append(bytes, '\r', '\n')                       // 長さの後のCRLF
    bytes = append(bytes, v.bulk...)                         // データ本体を追加
    bytes = append(bytes, '\r', '\n')                        // データ本体の後のCRLF

    return bytes
}
```

**RESP 形式:** `$バイト数\r\nデータ本体\r\n`

**処理の詳細:**

1. **プレフィックス**: `BULK`定数（`$`）を追加
2. **長さ情報**: `strconv.Itoa(len(v.bulk))`でバイト数を文字列に変換
3. **長さの終端**: 長さ情報の後に`\r\n`を追加
4. **データ本体**: `v.bulk`の内容を追加
5. **データの終端**: データ本体の後に`\r\n`を追加

**例:**

- 入力: `Value{typ: "bulk", bulk: "hello"}`
- 出力: `$5\r\nhello\r\n`

#### 6.2.4 Array（配列）のシリアライズ

```go
func (v Value) marshalArray() []byte {
    len := len(v.array)
    var bytes []byte
    bytes = append(bytes, ARRAY)                    // '*' を追加
    bytes = append(bytes, strconv.Itoa(len)...)    // 要素数を追加
    bytes = append(bytes, '\r', '\n')              // 要素数の後のCRLF

    for i := 0; i < len; i++ {
        bytes = append(bytes, v.array[i].Marshal()...)  // 各要素を再帰的にシリアライズ
    }

    return bytes
}
```

**RESP 形式:** `*要素数\r\n[要素1のRESP表現][要素2のRESP表現]...`

**処理の詳細:**

1. **プレフィックス**: `ARRAY`定数（`*`）を追加
2. **要素数**: `strconv.Itoa(len)`で要素数を文字列に変換
3. **要素数の終端**: `\r\n`を追加
4. **要素の処理**: 各要素に対して再帰的に`Marshal()`を呼び出し

**再帰処理の重要性:**

- 配列の要素は、バルク文字列や別の配列など、任意の型である可能性がある
- 各要素の型を判定し、適切なシリアライズ処理を行う
- パーサーの再帰処理と対称的な関係

**例:**

- 入力: `Value{typ: "array", array: [Value{typ: "bulk", bulk: "PING"}, Value{typ: "bulk", bulk: "TEST"}]}`
- 出力: `*2\r\n$4\r\nPING\r\n$4\r\nTEST\r\n`

#### 6.2.5 Error（エラー）のシリアライズ

```go
func (v Value) marshallError() []byte {
    var bytes []byte
    bytes = append(bytes, ERROR)        // '-' を追加
    bytes = append(bytes, v.str...)    // エラーメッセージを追加
    bytes = append(bytes, '\r', '\n')  // CRLFを追加

    return bytes
}
```

**RESP 形式:** `-エラーメッセージ\r\n`

**使用例:**

- 入力: `Value{typ: "error", str: "ERR unknown command"}`
- 出力: `-ERR unknown command\r\n`

#### 6.2.6 Null（ヌル）のシリアライズ

```go
func (v Value) marshallNull() []byte {
    return []byte("$-1\r\n")
}
```

**RESP 形式:** `$-1\r\n`

**Null の特徴:**

- 固定の 5 バイト表現
- データが存在しないことを表す
- Redis では「キーが存在しない」などの場合に使用

### 6.3 Writer 構造体とメソッド

#### 6.3.1 Writer 構造体の定義

```go
type Writer struct {
    writer io.Writer
}

func NewWriter(w io.Writer) *Writer {
    return &Writer{writer: w}
}
```

**Writer 構造体の役割:**

- `io.Writer`インターフェースをラップ
- ネットワーク接続（`net.Conn`）への書き込みを抽象化
- コンストラクタ関数でインスタンスを作成

**`io.Writer`インターフェース:**

- Go 言語の標準的な書き込みインターフェース
- `Write([]byte) (int, error)`メソッドを持つ
- `net.Conn`、`os.File`、`bytes.Buffer`などが実装

#### 6.3.2 Write メソッドの実装

```go
func (w *Writer) Write(v Value) error {
    var bytes = v.Marshal()    // ValueをRESPバイト列に変換

    _, err := w.writer.Write(bytes)  // ネットワークに書き込み
    if err != nil {
        return err
    }

    return nil
}
```

**処理の流れ:**

1. **シリアライズ**: `v.Marshal()`で`Value`を RESP バイト列に変換
2. **書き込み**: `w.writer.Write(bytes)`でネットワーク接続に書き込み
3. **エラーハンドリング**: 書き込みエラーがあれば返す

**エラーハンドリング:**

- ネットワークエラー（接続切断など）
- 書き込み権限の問題
- バッファオーバーフローなど

### 6.4 Writer の使用例

#### 6.4.1 基本的な使用法

```go
// Writerの作成
writer := NewWriter(conn)

// Simple String "OK" を送信
writer.Write(Value{typ: "string", str: "OK"})

// Bulk String "hello" を送信
writer.Write(Value{typ: "bulk", bulk: "hello"})

// エラーメッセージを送信
writer.Write(Value{typ: "error", str: "ERR unknown command"})

// Nullを送信
writer.Write(Value{typ: "null"})
```

#### 6.4.2 main.go での実際の使用

```go
for {
    resp := NewResp(conn)
    value, err := resp.Read()
    if err != nil {
        fmt.Println(err)
        return
    }

    // 受信したデータを無視（現在は固定応答）
    _ = value

    // Writerを使って "OK" を送信
    writer := NewWriter(conn)
    writer.Write(Value{typ: "string", str: "OK"})
}
```

### 6.5 Reader と Writer の関係

**対称的な関係:**

- **Reader**: RESP バイト列 → `Value`構造体（デシリアライズ）
- **Writer**: `Value`構造体 → RESP バイト列（シリアライズ）

**データフロー:**

1. クライアントが RESP データを送信
2. Reader がパースして`Value`に変換
3. サーバーが処理（現在は固定応答）
4. Writer が`Value`を RESP バイト列に変換
5. クライアントにレスポンスを送信

**設計の利点:**

- 型安全なデータ処理
- コードの再利用性
- テストの容易さ
- 拡張性の確保

---

## Redis コマンドハンドラーの実装

これまでに **シリアライザ（Serializer）** を作成し、クライアントからコマンドを受け取ったあとにどのように応答するかを学びました。

ここからは **CommandsHandler** を構築し、いくつかの Redis コマンドを実際に実装していきます。

### 7.1 コマンドハンドラーの概要

**コマンドハンドラーの役割:**

- クライアントから受け取ったコマンドを解析
- コマンド名に応じて適切な処理関数を呼び出し
- 処理結果を RESP 形式でクライアントに返送

**リクエストの構造:**

- クライアントから受け取るリクエストは **RESP の配列（Array）** 形式
- 最初の要素が「コマンド名」
- 残りの要素が「引数」

**例: `SET name Ahmed` コマンドの場合:**

```go
Value{
    typ: "array",
    array: []Value{
        Value{typ: "bulk", bulk: "SET"},      // コマンド名
        Value{typ: "bulk", bulk: "name"},    // 引数1（キー）
        Value{typ: "bulk", bulk: "Ahmed"},   // 引数2（値）
    },
}
```

### 7.2 ハンドラーマップの定義

```go
var Handlers = map[string]func([]Value) Value{
    "PING": ping,
    "SET":  set,
    "GET":  get,
    "HSET": hset,
    "HGET": hget,
}
```

**ハンドラーマップの特徴:**

- コマンド名（大文字）をキーとして、対応する処理関数をマッピング
- Redis のコマンドは大文字小文字を区別しないため、大文字で統一
- 各ハンドラー関数は `[]Value`（引数の配列）を受け取り、`Value`（結果）を返す

### 7.3 PING コマンドの実装

**PING コマンドの仕様:**

- 引数なし: `PONG` を返す
- 引数あり: その引数をそのまま返す

```go
func ping(args []Value) Value {
    // 引数が提供されていない場合 (例: PING)
    if len(args) == 0 {
        return Value{typ: "string", str: "PONG"}
    }
    // 引数が提供された場合 (例: PING hello)
    return Value{typ: "string", str: args[0].bulk}
}
```

**使用例:**

- `PING` → `+PONG\r\n`
- `PING hello` → `+hello\r\n`

### 7.4 SET と GET コマンドの実装

#### 7.4.1 データストアの定義

```go
// SET/GET コマンド用のデータストア
var SETs = map[string]string{}
var SETsMu = sync.RWMutex{} // 並行アクセス制御用
```

**データストアの特徴:**

- `map[string]string`: キーと値のペアを保存
- `sync.RWMutex`: 複数のゴルーチンからの同時アクセスを制御
- 読み取り操作は並行実行可能、書き込み操作は排他的

#### 7.4.2 SET コマンド

```go
func set(args []Value) Value {
    // 引数の数（キーと値の2つ）が正しいか検証
    if len(args) != 2 {
        return Value{typ: "error", str: "ERR wrong number of arguments for 'set' command"}
    }

    key := args[0].bulk   // キー
    value := args[1].bulk // 値

    // 書き込み操作のため排他ロックを取得
    SETsMu.Lock()
    SETs[key] = value
    SETsMu.Unlock()

    return Value{typ: "string", str: "OK"}
}
```

**SET コマンドの処理:**

1. 引数の数を検証（2 つ必要）
2. キーと値を抽出
3. 排他ロックを取得してデータストアに保存
4. 成功応答 `OK` を返す

#### 7.4.3 GET コマンド

```go
func get(args []Value) Value {
    // 引数の数（キーの1つ）が正しいか検証
    if len(args) != 1 {
        return Value{typ: "error", str: "ERR wrong number of arguments for 'get' command"}
    }

    key := args[0].bulk

    // 読み取り操作のため読み取りロックを取得
    SETsMu.RLock()
    value, ok := SETs[key]
    SETsMu.RUnlock()

    // キーが存在しなかった場合
    if !ok {
        return Value{typ: "null"}
    }

    // 値が存在した場合、Bulk Stringとして返す
    return Value{typ: "bulk", bulk: value}
}
```

**GET コマンドの処理:**

1. 引数の数を検証（1 つ必要）
2. キーを抽出
3. 読み取りロックを取得してデータストアから検索
4. キーが存在しない場合は `null` を返す
5. キーが存在する場合は値を `bulk` として返す

### 7.5 HSET と HGET コマンドの実装

#### 7.5.1 ハッシュデータストアの定義

```go
// HSET/HGET コマンド用のデータストア
var HSETs = map[string]map[string]string{}
var HSETsMu = sync.RWMutex{}
```

**ハッシュデータストアの特徴:**

- `map[string]map[string]string`: ハッシュ名 → フィールドと値のマップ
- Redis の Hash 型を模倣
- 二重構造でネストしたハッシュを実現

**データ構造の例:**

```go
{
    "users": {
        "u1": "Ahmed",
        "u2": "Mohamed",
    },
    "posts": {
        "p1": "Hello World",
        "p2": "Welcome to my blog",
    },
}
```

#### 7.5.2 HSET コマンド

```go
func hset(args []Value) Value {
    // 引数の数（ハッシュ名、キー、値の3つ）が正しいか検証
    if len(args) != 3 {
        return Value{typ: "error", str: "ERR wrong number of arguments for 'hset' command"}
    }

    hash := args[0].bulk  // ハッシュ名（例: "users"）
    key := args[1].bulk   // フィールドキー（例: "u1"）
    value := args[2].bulk // 値（例: "Ahmed"）

    HSETsMu.Lock()
    // ハッシュ名がまだ存在しない場合、新しい内部マップを作成
    if _, ok := HSETs[hash]; !ok {
        HSETs[hash] = map[string]string{}
    }
    // 指定されたハッシュの内部マップにキーと値を保存
    HSETs[hash][key] = value
    HSETsMu.Unlock()

    return Value{typ: "string", str: "OK"}
}
```

**HSET コマンドの処理:**

1. 引数の数を検証（3 つ必要）
2. ハッシュ名、フィールドキー、値を抽出
3. 排他ロックを取得
4. ハッシュが存在しない場合は新規作成
5. ハッシュ内のフィールドに値を設定
6. 成功応答 `OK` を返す

**使用例:**

- `HSET users u1 Ahmed` → ユーザー "Ahmed" を ID "u1" で保存
- `HSET posts p1 "Hello World"` → 投稿 "Hello World" を ID "p1" で保存

#### 7.5.3 HGET コマンド

```go
func hget(args []Value) Value {
    // 引数の数（ハッシュ名、キーの2つ）が正しいか検証
    if len(args) != 2 {
        return Value{typ: "error", str: "ERR wrong number of arguments for 'hget' command"}
    }

    hash := args[0].bulk // ハッシュ名
    key := args[1].bulk  // フィールドキー

    HSETsMu.RLock()
    // 指定されたハッシュの内部マップから値を取得
    value, ok := HSETs[hash][key]
    HSETsMu.RUnlock()

    // キーが存在しなかった場合（ハッシュ自体が存在しない場合も含む）
    if !ok {
        return Value{typ: "null"}
    }

    // 値が存在した場合、Bulk Stringとして返す
    return Value{typ: "bulk", bulk: value}
}
```

**HGET コマンドの処理:**

1. 引数の数を検証（2 つ必要）
2. ハッシュ名とフィールドキーを抽出
3. 読み取りロックを取得してハッシュ内のフィールドを検索
4. フィールドが存在しない場合は `null` を返す
5. フィールドが存在する場合は値を `bulk` として返す

**使用例:**

- `HGET users u1` → "Ahmed" を返す
- `HGET posts p1` → "Hello World" を返す

### 7.6 main.go でのコマンド処理

```go
for {
    // --- リクエストの読み取りとパース ---
    resp := NewResp(conn)
    value, err := resp.Read()
    if err != nil {
        fmt.Println(err)
        return
    }

    // --- リクエストの検証 ---
    // Redisコマンドは必ずRESP Array（配列）である必要があります
    if value.typ != "array" {
        fmt.Println("Invalid request, expected array")
        continue
    }

    // 配列が空であってはなりません（最低でもコマンド名が必要）
    if len(value.array) == 0 {
        fmt.Println("Invalid request, expected array length > 0")
        continue
    }

    // --- コマンド名と引数の抽出 ---
    // 配列の最初の要素がコマンド名です。それを大文字に変換します
    command := strings.ToUpper(value.array[0].bulk)
    // 配列の2番目以降の要素すべてを引数（args）としてスライス
    args := value.array[1:]

    // --- コマンドの実行と応答 ---
    writer := NewWriter(conn)

    // Handlersマップから、コマンド名に対応するハンドラー関数を検索
    handler, ok := Handlers[command]
    if !ok {
        // コマンドが見つからなかった場合
        fmt.Println("Invalid command: ", command)
        // エラー応答をクライアントに返します
        writer.Write(Value{typ: "error", str: fmt.Sprintf("ERR unknown command '%s'", command)})
        continue
    }

    // ハンドラー関数を実行し、引数（args）を渡して、結果（RESP Value）を受け取ります
    result := handler(args)

    // 実行結果（Value）を Writer.Write() で RESP バイト列に変換し、クライアントに送信
    writer.Write(result)
}
```

**コマンド処理の流れ:**

1. **リクエスト読み取り**: RESP パーサーでクライアントからのデータを解析
2. **リクエスト検証**: 配列形式であることを確認
3. **コマンド抽出**: 最初の要素をコマンド名、残りを引数として抽出
4. **ハンドラー検索**: コマンド名に対応するハンドラー関数を検索
5. **コマンド実行**: ハンドラー関数を実行して結果を取得
6. **レスポンス送信**: Writer で結果を RESP 形式に変換してクライアントに送信

### 7.7 並行処理とスレッドセーフティ

**RWMutex の使用理由:**

- 複数のクライアントが同時に接続する可能性を考慮
- 読み取り操作は並行実行可能（`RLock()`）
- 書き込み操作は排他的（`Lock()`）
- データの整合性を保証

**ロックの取得と解放:**

- `Lock()` / `Unlock()`: 書き込み操作用
- `RLock()` / `RUnlock()`: 読み取り操作用
- `defer` を使った確実なロック解放

### 7.8 エラーハンドリング

**引数検証:**

- 各コマンドで必要な引数の数をチェック
- 不正な引数数の場合はエラー応答を返す

**未知のコマンド:**

- ハンドラーマップに存在しないコマンドの場合
- `ERR unknown command` エラーを返す

**データの存在確認:**

- GET/HGET でキーが存在しない場合
- `null` 応答を返す（Redis の標準的な動作）

---

## 8. 実行方法とテスト

### 8.1 サーバーの起動

```bash
# プロジェクトディレクトリに移動
cd /path/to/build-your-own-redis-go

# Goプログラムを実行
go run main.go resp.go handler.go
```

**期待される出力:**

```
Listening on port :6379
```

### 8.2 クライアントでのテスト

**別のターミナルで redis-cli を使用:**

```bash
# Redisクライアントで接続
redis-cli -p 6379

# コマンドを送信
127.0.0.1:6379> PING
PONG
127.0.0.1:6379> PING hello
hello
127.0.0.1:6379> SET name Ahmed
OK
127.0.0.1:6379> GET name
Ahmed
127.0.0.1:6379> GET nonexistent
(nil)
127.0.0.1:6379> HSET users u1 Ahmed
OK
127.0.0.1:6379> HGET users u1
Ahmed
127.0.0.1:6379> HGET users u2
(nil)
```

**サーバー側の出力例:**

```
{array [{bulk PING}]}
{array [{bulk PING} {bulk hello}]}
{array [{bulk SET} {bulk name} {bulk Ahmed}]}
{array [{bulk GET} {bulk name}]}
{array [{bulk GET} {bulk nonexistent}]}
{array [{bulk HSET} {bulk users} {bulk u1} {bulk Ahmed}]}
{array [{bulk HGET} {bulk users} {bulk u1}]}
{array [{bulk HGET} {bulk users} {bulk u2}]}
```

### 8.3 telnet でのテスト

```bash
# telnetで接続
telnet localhost 6379

# RESP形式でコマンドを送信
*3
$3
SET
$4
name
$5
Ahmed
```

**期待される応答:**

```
+OK
```

---

## 9. トラブルシューティング

### 9.1 よくある問題

**ポートが既に使用されている:**

```
listen tcp :6379: bind: address already in use
```

- 解決方法: 他の Redis サーバーが動いている場合は停止するか、別のポートを使用

**権限不足:**

```
listen tcp :6379: bind: permission denied
```

- 解決方法: 管理者権限で実行するか、1024 番以上のポートを使用

**接続が拒否される:**

```
dial tcp 127.0.0.1:6379: connect: connection refused
```

- 解決方法: サーバーが起動しているか確認

### 9.2 デバッグのヒント

1. **サーバー側のログを確認**: 受信したデータが正しくパースされているか
2. **ネットワーク接続を確認**: `netstat -an | grep 6379`でポートの状態を確認
3. **クライアント側のエラーを確認**: redis-cli のエラーメッセージを確認

### 9.3 パフォーマンスの考慮事項

- この実装は教育目的のため、パフォーマンスは最適化されていません
- 実際の本番環境では、並行処理（ゴルーチン）やコネクションプールの使用を検討
- バッファサイズの調整やメモリプールの使用も有効

---

## 10. 参考資料・関連リンク

- **[Build Redis from scratch](https://www.build-redis-from-scratch.dev/en/introduction)**: このプロジェクトの主要な参考資料
- **[Writing RESP](https://www.build-redis-from-scratch.dev/en/resp-writer)**: RESP Writer の実装に関する詳細なチュートリアル
- **[Redis Commands](https://www.build-redis-from-scratch.dev/en/redis-commands)**: Redis コマンドハンドラーの実装に関するチュートリアル
- **[Redis Protocol Specification](https://redis.io/docs/latest/develop/reference/protocol-spec/)**: RESP プロトコルの公式仕様
- **[Go net package documentation](https://pkg.go.dev/net)**: Go 言語のネットワークプログラミング
- **[bufio package documentation](https://pkg.go.dev/bufio)**: Go 言語のバッファリング I/O
- **[sync package documentation](https://pkg.go.dev/sync)**: Go 言語の並行処理と同期

### さらなる学習のために

このプロジェクトは、Redis の内部動作を理解するための第一歩です。より高度な機能を実装したい場合は、以下の要素を追加することを検討してください：

- **データ永続化**: AOF（Append Only File）や RDB ファイルの実装
- **複数データ型**: 文字列、ハッシュ、リスト、セット、ソート済みセットのサポート
- **並行処理**: ゴルーチンを使った複数接続の同時処理
- **メモリ管理**: 効率的なデータ構造とメモリ使用量の最適化
- **コマンド処理**: 実際の Redis コマンドの実装
