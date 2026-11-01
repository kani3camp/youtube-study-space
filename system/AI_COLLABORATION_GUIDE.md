# YouTube Study Space - AI協業ガイド（system）

> [!IMPORTANT]
> リポジトリ共通のAI協業ルールは [`../AGENTS.md`](../AGENTS.md) が正です。認可、ブランチ/PR運用、検証、安全性についてこの文書と食い違う場合は `AGENTS.md` に従ってください。この文書は `system/` 固有の実装案内だけを扱います。

## `system/` の役割

`system/` は Go バックエンド、定期ジョブ、Lambda/Fargate エントリポイントを含みます。

- `cmd/youtube-bot/`: YouTube Live Chat をポーリングする実運用Bot
- `cmd/batch/`: Fargate 日次バッチ
- `cmd/lambda/`: 定期・補助 Lambda
- `core/workspaceapp/`: コマンド処理、席管理、休憩、バリデーション、バッチロジック
- `core/repository/`: Firestore データアクセス
- `core/youtubebot/`: YouTube Live Chat API
- `core/guardians/`: ライブ監視
- `core/moderatorbot/`: Discord/モデレーション
- `core/mybigquery/`: BigQuery
- `core/i18n/`: locale と型付き翻訳ラッパー
- `internal/`: AWS runtime、管理者操作、logging など

Go/toolchain と依存バージョンは [`go.mod`](./go.mod) を正とし、この文書には固定値を重複させません。

## 安全な検証と実サービス起動を分ける

通常のコード検証:

```bash
cd system
go test -shuffle=on ./...
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
```

Firestore Emulator の意味論が必要な変更では、リポジトリルートから:

```bash
bash .github/scripts/run-firestore-integration-tests.sh
```

`go run ./cmd/youtube-bot` は通常のテストではありません。設定済みの Google/YouTube/Discord 等へ接続し、チャット応答・通知・Firestore 状態変更を行い得ます。実環境スモークテストが明示的に依頼された場合だけ使用し、起動時に表示される Google Cloud Project ID が対象環境であることを確認してください。

## i18n

- locale ファイル: [`core/i18n/locales/`](./core/i18n/locales/)。現在の実ファイルを正とし、言語一覧を文書へ固定しません。
- メタ情報: [`core/i18n/meta/i18n_meta.toml`](./core/i18n/meta/i18n_meta.toml)
- 生成物: `core/i18n/typed/zz_generated.i18n_messages.go`
- 生成: `I18N_BASELINE=ja go generate ./...`

locale/metadata/generator を変更した場合は生成物の差分も確認してください。

## 重要な契約

- 席状態・割当ロジック: [`../formal-spec/README.md`](../formal-spec/README.md) と [`../formal-spec/fsl/README.md`](../formal-spec/fsl/README.md)
- Firestore Emulator / Rules の検証区分: [`../firebase/README.md`](../firebase/README.md)
- 保存・転送・削除・retention: [`../docs/privacy/`](../docs/privacy/)
- システム全体と monitor の席数制御: [`../docs/development/architecture.md`](../docs/development/architecture.md)

形式仕様の一部は実装との照合が接続済み、一部は未接続です。CI を通すためだけに仕様を弱めないでください。

## Go toolchain とベースイメージ

Go の minor/major を上げる場合は、`system/go.mod` と `Dockerfile.lambda` / `Dockerfile.fargate` の Go builder tag を同一 minor に揃えます。

具体的な digest の取得方法、Dependabot PR の確認、dev/prod への適用順序を含む **base image 更新運用の詳細な正本は [`README.md`](./README.md) の「base image 更新運用」**です。このAIガイドでは詳細手順を重複させません。
