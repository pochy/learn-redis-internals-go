package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Aof構造体: AOFファイルの操作を管理します。
type Aof struct {
	file *os.File      // ディスク上のファイルオブジェクト
	rd   *bufio.Reader // ファイルから効率的に読み取るためのリーダー
	mu   sync.Mutex    // ファイルへの書き込みを排他的にするためのMutex
}

// NewAof: AOF構造体の新しいインスタンスを作成し、ファイルを開き、同期ゴルーチンを開始します。
func NewAof(path string) (*Aof, error) {
	// ファイルが存在しなければ作成し、読み書きモードで開きます。パーミッションは 0666。
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	aof := &Aof{
		file: f,
		// ファイルオブジェクトfを元に、読み取り用のバッファ付きリーダーを作成します。
		rd: bufio.NewReader(f),
	}

	// 永続性を高めるため、1秒ごとにファイルをディスクに同期するゴルーチン（並行処理）を開始します。
	go func() {
		for {
			aof.mu.Lock()
			// aof.file.Sync() はメモリ上のバッファを強制的にディスクに書き込みます。
			err := aof.file.Sync()
			if err != nil {
				fmt.Println("Error syncing AOF file:", err)
			}
			aof.mu.Unlock()

			// 1秒間待機します。
			time.Sleep(time.Second)
		}
	}()

	return aof, nil
}

// Close: ファイルを閉じ、Mutexを安全に解放します。
func (aof *Aof) Close() error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	return aof.file.Close()
}

// Write: ValueオブジェクトをRESPバイト列に変換し、AOFファイルに追記します。
func (aof *Aof) Write(value Value) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	// value.Marshal() でValueをRESP形式のバイト列に変換します。
	_, err := aof.file.Write(value.Marshal())
	if err != nil {
		return err
	}

	return nil
}

// Read: AOFファイルの内容をRESP形式として読み取り、読み取ったコマンドごとにコールバック関数を実行します。
func (aof *Aof) Read(callback func(value Value)) error {
	// AOFファイルの読み取り中は書き込みを禁止します。
	aof.mu.Lock()
	defer aof.mu.Unlock()

	// ファイルの読み取りを最初から開始するために、ポインターを先頭（オフセット0）に戻します。
	_, err := aof.file.Seek(0, 0)
	if err != nil {
		return err
	}

	// ファイルリーダーを使って新しいRESPパーサーを作成します。
	resp := NewResp(aof.file)

	// EOF（ファイルの終端）に達するまでループし、コマンドを一つずつ読み取ります。
	for {
		// RESPパーサーを使ってファイルから次のValue（コマンド）を読み取ります。
		value, err := resp.Read()

		// 正常に読み込めた場合
		if err == nil {
			callback(value)
		}

		// ファイルの終端に達した場合、これは正常な終了（"good error"）とみなし、ループを抜けます。
		if err == io.EOF {
			break
		}

		// その他のエラー（ファイル破損など）が発生した場合は、エラーを呼び出し元に返します。
		if err != nil {
			return err
		}
	}

	return nil
}
