# room-image-prompt

ルーム画像生成（画像生成モデル等）向けのプロンプトを組み立て、`output/` に保存する Go CLI です。候補テキストとテンプレートは `data/` に置き、`go:embed` でバイナリに同梱します（外部の `-data-dir` はありません。差し替えは `data/` を編集して再ビルドしてください）。

## 必要環境

- Go 1.25.0 以上（リポジトリの `system/go.mod` に合わせています）

## ビルド・実行

```bash
cd tools/room-image-prompt
go build -o room-image-prompt ./cmd/room-image-prompt
./room-image-prompt -version
```

引数なしで、5段階のテーマを乱数で選び `data/prompt_template.txt` と連結した UTF-8 テキストを **`output/prompt-<タイムスタンプ>.txt`** に書き込み、**保存したファイルの絶対パスを標準出力に1行**出します。実行時は **`tools/room-image-prompt` をカレントディレクトリにした状態**で動かす想定です（省略時の出力先が `./output/` になるため）。

### オプション

| フラグ | 説明 |
|--------|------|
| `-version` | バージョン表示して終了 |
| `-out <path>` | 出力ファイル（省略時は上記タイムスタンプ名） |
| `-seed <uint64>` | 乱数シード（10進）。省略時は非固定 |

開発中は `go run` でも可です。

```bash
cd tools/room-image-prompt
go run ./cmd/room-image-prompt -version
go run ./cmd/room-image-prompt -seed 1
```

## テスト

```bash
cd tools/room-image-prompt
go test ./...
```

## ライセンス

リポジトリ全体のライセンスに従います。
