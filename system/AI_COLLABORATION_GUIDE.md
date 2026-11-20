# YouTube Study Space - AI協業ガイド（system）

> [!IMPORTANT]
> リポジトリ共通のAI協業ルール（認可、ブランチ/PR運用、レビュー基準、通常検証コマンド、Task-specific Contracts、安全性）は [`../AGENTS.md`](../AGENTS.md) が正です。この文書は `AGENTS.md` と重複しない `system/` 固有の構成・生成物・ベースイメージ案内だけを扱います。

## `system/` の役割とパッケージ構成

`system/` は Go バックエンド、定期ジョブ、Lambda/Fargate エントリポイント、運用・監査CLIを含みます。

- `cmd/youtube-bot/`: YouTube Live Chat をポーリングする実運用Bot（`go run ./cmd/youtube-bot` は実環境操作であり通常の検証コマンドではない）
- `cmd/batch/`: Fargate 日次バッチ
- `cmd/lambda/`: 定期・補助 Lambda
- `cmd/i18n-gen/`: i18n 型付きラッパー生成ツール
- `cmd/*-admin/`, `cmd/*-audit/`: プライバシー・データ移行・監査向けの個別運用CLI
- `core/workspaceapp/`: コマンド処理、席管理、休憩、バリデーション、バッチロジック
- `core/repository/`: Firestore データアクセス
- `core/youtubebot/`: YouTube Live Chat API
- `core/guardians/`: ライブ監視
- `core/moderatorbot/`: Discord/モデレーション
- `core/mybigquery/`, `core/mystorage/`: BigQuery / Cloud Storage
- `core/i18n/`: locale と型付き翻訳ラッパー
- `core/timeutil/`, `core/utils/`, `core/studyspaceerror/`, `core/wordsreader/`: 日時計算・コマンド解析・共通エラー・NGワード読込
- `internal/`: AWS runtime、管理者操作、logging など

Go/toolchain と依存バージョンは [`go.mod`](./go.mod) を正とし、この文書には固定値を重複させません。検証コマンドと契約文書の一覧は [`../AGENTS.md`](../AGENTS.md) を参照してください。

## i18n

- locale ファイル: [`core/i18n/locales/`](./core/i18n/locales/)。現在の実ファイルを正とし、言語一覧を文書へ固定しません。
- メタ情報: [`core/i18n/meta/i18n_meta.toml`](./core/i18n/meta/i18n_meta.toml)
- 生成物: `core/i18n/typed/zz_generated.i18n_messages.go`
- 生成: `I18N_BASELINE=ja go generate ./...`

locale/metadata/generator を変更した場合は生成物の差分も確認してください。

## Go toolchain とベースイメージ

Go の minor/major を上げる場合は、`system/go.mod` と `Dockerfile.lambda` / `Dockerfile.fargate` の Go builder tag を同一 minor に揃えます。

具体的な digest の取得方法、Dependabot PR の確認、dev/prod への適用順序を含む **base image 更新運用の詳細な正本は [`README.md`](./README.md) の「base image 更新運用」**です。このAIガイドでは詳細手順を重複させません。
