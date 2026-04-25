# room-image-prompt

ルーム画像生成（画像生成モデル等）向けのプロンプトを組み立てて標準出力に出すための Go CLI です。開発初期のスケルトンです。

## 必要環境

- Go 1.25.0 以上（リポジトリの `system/go.mod` に合わせています）

## ビルド・実行

```bash
cd tools/room-image-prompt
go build -o room-image-prompt ./cmd/room-image-prompt
./room-image-prompt -version
```

開発中は `go run` でも可です。

```bash
go run ./cmd/room-image-prompt -version
```

## ライセンス

リポジトリ全体のライセンスに従います。
