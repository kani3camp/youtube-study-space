// room-image-prompt はルーム画像生成（画像生成AI等）向けのプロンプトを組み立て・出力するCLIです。
package main

import (
	"flag"
	"fmt"
	"os"
)

const usageText = `room-image-prompt — ルーム画像生成用プロンプトを生成するCLI

使い方:
  room-image-prompt [オプション]

オプション:
`

func main() {
	fs := flag.NewFlagSet("room-image-prompt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usageText)
		fs.PrintDefaults()
	}

	version := fs.Bool("version", false, "バージョンを表示して終了")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if *version {
		fmt.Println("room-image-prompt 0.1.0 (dev)")
		return
	}

	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "不明な引数: %q\n", fs.Arg(0))
		fs.Usage()
		os.Exit(2)
	}

	// TODO: レイアウト・スタイル入力からプロンプトを組み立てて標準出力へ
	fmt.Fprintln(os.Stderr, "まだサブコマンド・生成ロジックは未実装です。-version で動作確認できます。")
	fs.Usage()
	os.Exit(1)
}
