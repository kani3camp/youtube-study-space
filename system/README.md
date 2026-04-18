
## i18n翻訳関数の自動生成

翻訳文言（TOML）とメタファイル（TOML）から、型付きラッパー関数を自動生成して利用します。
目的は「引数個数・型のミスをコンパイル時に検出」することです。

- 言語ごとのロケールファイル: `core/i18n/locales/*.toml`
  - 例:
    ```toml
    [common]
    sir = "こんにちは、{0}さん"

    [command]
    exit = "{0}さんは、席番号{1}で{2}分作業しました。お疲れ様でした。"
    ```
- 全言語共通メタファイル: `core/i18n/meta/i18n_meta.toml`
  - ロケールファイルで使用するキーと引数（型指定含む）を定義
  - 例:
    ```toml
    [common]
    sir = ["username: string"]

    [command]
    exit = ["username: string", "seat: int", "workedMin: int"]
    ```
- 生成物: `core/i18n/typed/zz_generated.i18n_messages.go`（パッケージ `i18nmsg`）

設計のポイント:
- 生成コードは `internal/engine` を使用します（`engine.TranslateDefault(...)`）。
- アプリ側は必ず型安全な `i18nmsg.*` を使用してください。
- ロケールは `//go:embed` によりバイナリに埋め込み、`LoadLocaleFolderFS()` で読み込みます。

生成:
```bash
go generate ./...
```


## テスト用mockファイルの作成
使用ツール：https://github.com/uber-go/mock

### systemディレクトリに移動する
```shell
cd system
```

### mockファイルを作成する
**system ディレクトリで** `go generate ./...` を実行してください（CIと同じ手順で、モック生成もここに統合しています）。

```shell
go generate ./...
```


## 日次バッチ（ECS Fargate）と通知（SNS→Lambda→Discord）

- 実行基盤: AWS ECS Fargate (arm64) 上の単一バッチコンテナ
- オーケストレーション: AWS Step Functions（直列実行）
- スケジュール: 00:00:15 JST に開始（EventBridge → Step Functions）
- 実行順序: `reset-daily-total` → `update-rp` → `transfer-bq`
- 認証情報: DynamoDB `secrets` テーブルからGCP SA JSON取得
- ネットワーク: Public Subnet, Public IP割当, DynamoDB Gateway VPC Endpoint
- ログ: CloudWatch Logs（ECS/Step Functions/Lambda）
- 通知: CloudWatch Alarm/SFN失敗 → SNS → `sns_notify_discord` Lambda → Discord

### ビルド/イメージ
- Fargateバッチ: `system/Dockerfile.fargate`
- Lambda群: `system/Dockerfile.lambda`

### 手動実行（ローカル確認用）
```bash
# Fargate用バッチのローカルビルド例（arm64）
docker buildx build --platform linux/arm64 -f system/Dockerfile.fargate system --load
```

### base image 更新運用

`Dockerfile.lambda` / `Dockerfile.fargate` の `FROM` は、`image:tag@sha256:...` の形式で **digest 固定** している（再現可能ビルドのため。詳細は issue #693）。digest の更新は基本的に Dependabot の docker ecosystem PR に任せる。

- **Dependabot からの digest 更新 PR が来たとき**:
  1. `aws-cdk/` で `pnpm cdk:diff --profile <dev プロファイル>` を実行し、変更が digest 差し替えだけであることを確認
  2. `pnpm cdk:deploy --profile <dev プロファイル>` で dev 環境にデプロイしてスモーク確認
  3. 問題なければ prod プロファイルで同じ手順を実行
  4. プロファイル切り替えの詳細は [`aws-cdk/README.md`](../aws-cdk/README.md) を参照
- **手動で base image を更新したいとき**（Go の minor 上げ、セキュリティパッチの即時適用等）:
  ```bash
  # タグに対する最新 digest を取得
  docker buildx imagetools inspect docker.io/library/golang:1.25 --format '{{.Manifest.Digest}}'
  docker buildx imagetools inspect public.ecr.aws/lambda/provided:al2023 --format '{{.Manifest.Digest}}'
  docker buildx imagetools inspect gcr.io/distroless/static-debian12:nonroot --format '{{.Manifest.Digest}}'
  ```
  取得した `sha256:...` を Dockerfile の `FROM ...@sha256:...` に差し替えて PR を出す。
- **Go の minor / major を上げる場合**は、`go.mod` の `go x.yy` と Dockerfile の `golang:x.yy@sha256:...` のタグを同一 minor に揃えること。詳細は [`AI_COLLABORATION_GUIDE.md`](./AI_COLLABORATION_GUIDE.md) の「Go toolchain とベースイメージのバージョン整合」を参照。
