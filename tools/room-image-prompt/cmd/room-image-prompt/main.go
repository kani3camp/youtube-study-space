// room-image-prompt はルーム画像生成（画像生成AI等）向けのプロンプトを組み立て・出力するCLIです。
package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kani3camp/youtube-study-space/tools/room-image-prompt/data"
	"github.com/kani3camp/youtube-study-space/tools/room-image-prompt/internal/theme"
)

const (
	versionString = "room-image-prompt 0.1.0 (dev)"
	usageText     = `room-image-prompt — ルーム画像生成用プロンプトを生成するCLI

使い方:
  room-image-prompt [オプション]

引数なしで、同梱データと座席数（10〜20）の乱数をテンプレに連結した結果を output/ に保存し、
保存したファイルの絶対パスを標準出力に1行で出します。

オプション:
`
)

func main() {
	err := run()
	if err != nil {
		var ec exitCode
		if errors.As(err, &ec) {
			os.Exit(int(ec))
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("room-image-prompt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		if _, err := fmt.Fprint(fs.Output(), usageText); err != nil {
			return
		}
		fs.PrintDefaults()
	}

	version := fs.Bool("version", false, "バージョンを表示して終了")
	outPath := fs.String("out", "", "出力ファイルパス（省略時はカレントの output/prompt-<タイムスタンプ>.txt）")
	seedStr := fs.String("seed", "", "乱数シード（10進 uint64）。省略時は非固定")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if *version {
		fmt.Println(versionString)
		return nil
	}

	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "不明な引数: %q\n", fs.Arg(0))
		fs.Usage()
		return exitCode(2)
	}

	fsys := data.FS
	rng, err := newRNG(*seedStr)
	if err != nil {
		return err
	}

	th, err := theme.BuildTheme(fsys, rng)
	if err != nil {
		return err
	}
	tmpl, err := theme.ReadTemplate(fsys)
	if err != nil {
		return err
	}
	body := theme.RenderFinal(tmpl, th.FormatThemeBlock())

	dest := *outPath
	if dest == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("カレントディレクトリ: %w", err)
		}
		name := fmt.Sprintf("prompt-%s.txt", time.Now().Format("20060102150405"))
		dest = filepath.Join(wd, "output", name)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("出力ディレクトリ作成: %w", err)
	}
	if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
		return fmt.Errorf("出力書き込み: %w", err)
	}

	abs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("絶対パス解決: %w", err)
	}
	fmt.Println(abs)
	return nil
}

func newRNG(seedStr string) (*rand.Rand, error) {
	if seedStr != "" {
		s, err := strconv.ParseUint(seedStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("-seed: %w", err)
		}
		return rand.New(rand.NewPCG(s, 0)), nil
	}
	var buf [16]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return nil, fmt.Errorf("乱数シード生成: %w", err)
	}
	hi := binary.LittleEndian.Uint64(buf[:8])
	lo := binary.LittleEndian.Uint64(buf[8:])
	return rand.New(rand.NewPCG(hi, lo)), nil
}

type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit %d", e) }
