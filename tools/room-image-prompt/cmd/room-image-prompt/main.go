// room-image-prompt はルーム画像生成（画像生成AI等）向けのプロンプトを組み立て・出力するCLIです。
package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"github.com/kani3camp/youtube-study-space/tools/room-image-prompt/data"
	lookprofile "github.com/kani3camp/youtube-study-space/tools/room-image-prompt/internal/look"
	"github.com/kani3camp/youtube-study-space/tools/room-image-prompt/internal/theme"
)

const (
	versionString = "room-image-prompt 0.2.0 (dev)"
	usageText     = `room-image-prompt — ルーム画像生成用プロンプトを生成するCLI

使い方:
  room-image-prompt [オプション]

引数なしで、同梱データと座席数（10〜15）の乱数をテンプレに連結した結果を output/ に保存し、
同じ本文をクリップボードへコピーします。標準エラーに出力ファイル名とコピー成否、標準出力に
保存したファイルの絶対パスを1行で出します。

オプション:
`
	defaultOutputMaxAttempts = 10
	legacyStyleName          = "legacy"
)

func main() {
	err := run()
	if err != nil {
		var ec exitCode
		if errors.As(err, &ec) {
			os.Exit(int(ec))
		}
		if printErr := printError(os.Stderr, err); printErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("room-image-prompt", flag.ContinueOnError)
	configureUsage(fs, io.Discard)

	version := fs.Bool("version", false, "バージョンを表示して終了")
	outPath := fs.String("out", "", "出力ファイルパス（省略時はカレントの output/prompt-<タイムスタンプ>.txt）")
	seedStr := fs.String("seed", "", "乱数シード（10進 uint64）。省略時は非固定")
	styleName := fs.String("style", "", "内蔵スタイル名（legacy または生成済み direction-*）。省略時は legacy")
	styleFile := fs.String("style-file", "", "任意スタイルを読み込む UTF-8 テキストファイル（-style と同時指定不可）")
	lookName := fs.String("look", "", "内蔵Look名（auto / none / bundled look）。省略時は auto")
	lookFile := fs.String("look-file", "", "任意Lookを読み込む UTF-8 テキストファイル（-look と同時指定不可）")

	if err := fs.Parse(os.Args[1:]); err != nil {
		configureUsage(fs, os.Stderr)
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		if printErr := printError(fs.Output(), err); printErr != nil {
			return printErr
		}
		fs.Usage()
		return exitCode(2)
	}
	configureUsage(fs, os.Stderr)

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
		return fmt.Errorf("テーマ構築: %w", err)
	}
	tmpl, err := theme.ReadTemplate(fsys)
	if err != nil {
		return fmt.Errorf("テンプレート読込: %w", err)
	}
	style, err := resolveStyle(fsys, *styleName, *styleFile)
	if err != nil {
		return fmt.Errorf("スタイル読込: %w", err)
	}
	look, err := resolveLook(fsys, *lookName, *lookFile, rng)
	if err != nil {
		return fmt.Errorf("look読込: %w", err)
	}
	visualGuidance := composeVisualGuidance(style, look.Text)
	styledTemplate, err := theme.ApplyStyle(tmpl, visualGuidance)
	if err != nil {
		return fmt.Errorf("スタイル適用: %w", err)
	}
	body := theme.RenderFinal(styledTemplate, th.FormatThemeBlock())

	dest := *outPath
	if dest == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("カレントディレクトリ: %w", err)
		}
		dest, err = writeDefaultOutput(wd, body)
		if err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("出力ディレクトリ作成: %w", err)
		}
		if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
			return fmt.Errorf("出力書き込み: %w", err)
		}
	}

	abs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("絶対パス解決: %w", err)
	}

	base := filepath.Base(abs)
	fmt.Fprintf(os.Stderr, "出力: %s\n", base)
	if clipErr := clipboard.WriteAll(body); clipErr != nil {
		fmt.Fprintln(os.Stderr, "コピーに失敗しました")
	} else {
		fmt.Fprintln(os.Stderr, "クリップボードにコピーしました")
	}

	fmt.Println(abs)
	return nil
}

func resolveStyle(fsys fs.FS, styleName, styleFile string) (string, error) {
	if styleName != "" && styleFile != "" {
		return "", fmt.Errorf("-style と -style-file は同時指定できません")
	}

	if styleFile != "" {
		b, err := os.ReadFile(styleFile)
		if err != nil {
			return "", fmt.Errorf("-style-file %q: %w", styleFile, err)
		}
		return string(b), nil
	}

	switch styleName {
	case "", legacyStyleName:
		style, err := theme.ReadLegacyStyle(fsys)
		if err != nil {
			return "", fmt.Errorf("legacy style 読込: %w", err)
		}
		return style, nil
	default:
		if strings.HasPrefix(styleName, "direction-") {
			style, err := theme.ReadDirectionStyle(fsys, styleName)
			if err != nil {
				return "", fmt.Errorf("named style %q 読込: %w", styleName, err)
			}
			return style, nil
		}
		return "", fmt.Errorf("-style %q は未対応です（legacy または生成済み direction-* を指定してください）", styleName)
	}
}

func resolveLook(fsys fs.FS, lookName, lookFile string, rng *rand.Rand) (lookprofile.Selection, error) {
	if lookName != "" && lookFile != "" {
		return lookprofile.Selection{}, fmt.Errorf("-look と -look-file は同時指定できません")
	}

	if lookFile != "" {
		b, err := os.ReadFile(lookFile)
		if err != nil {
			return lookprofile.Selection{}, fmt.Errorf("-look-file %q: %w", lookFile, err)
		}
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		if strings.TrimSpace(text) == "" {
			return lookprofile.Selection{}, fmt.Errorf("-look-file %q: テキストが空です", lookFile)
		}
		return lookprofile.Selection{Name: "custom", Text: text}, nil
	}

	selection, err := lookprofile.Resolve(fsys, lookName, rng)
	if err != nil {
		return lookprofile.Selection{}, fmt.Errorf("bundled look解決: %w", err)
	}
	return selection, nil
}

func composeVisualGuidance(style, look string) string {
	if strings.TrimSpace(look) == "" {
		return style
	}
	style = strings.TrimRight(strings.ReplaceAll(style, "\r\n", "\n"), "\n")
	look = strings.TrimSpace(strings.ReplaceAll(look, "\r\n", "\n"))
	return style + "\n\n" + look + "\n"
}

func configureUsage(fs *flag.FlagSet, output io.Writer) {
	fs.SetOutput(output)
	fs.Usage = func() {
		if _, err := fmt.Fprint(fs.Output(), usageText); err != nil {
			return
		}
		fs.PrintDefaults()
	}
}

func printError(output io.Writer, err error) error {
	if _, writeErr := fmt.Fprintf(output, "%v\n", err); writeErr != nil {
		return fmt.Errorf("エラー出力: %w", writeErr)
	}
	return nil
}

func writeDefaultOutput(wd, body string) (string, error) {
	outputDir := filepath.Join(wd, "output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("出力ディレクトリ作成: %w", err)
	}

	for attempt := range defaultOutputMaxAttempts {
		dest := filepath.Join(outputDir, defaultOutputFileName(time.Now(), attempt))
		if err := writeFileExclusive(dest, body); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("出力書き込み: %w", err)
		}
		return dest, nil
	}
	return "", fmt.Errorf("出力ファイル名生成: %d 回連続で衝突しました", defaultOutputMaxAttempts)
}

func defaultOutputFileName(now time.Time, attempt int) string {
	suffix := fmt.Sprintf("%s-%09d", now.Format("20060102150405"), now.Nanosecond())
	if attempt > 0 {
		suffix = fmt.Sprintf("%s-%02d", suffix, attempt)
	}
	return fmt.Sprintf("prompt-%s.txt", suffix)
}

func writeFileExclusive(path, body string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("出力ファイル作成: %w", err)
	}

	n, writeErr := f.WriteString(body)
	if writeErr == nil && n != len(body) {
		writeErr = io.ErrShortWrite
	}
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("出力書き込み: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("出力ファイルを閉じる: %w", closeErr)
	}
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
