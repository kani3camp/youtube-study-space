# Go Backend (`system/`)

`system/` contains the Go backend, YouTube/Discord integrations, Firestore repository/application logic, Lambda handlers, and Fargate batch jobs. Repository-wide agent rules are in [`../AGENTS.md`](../AGENTS.md), and system-specific AI guidance is in [`AI_COLLABORATION_GUIDE.md`](./AI_COLLABORATION_GUIDE.md).

## Initial setup and safe verification

Use [`go.mod`](./go.mod) as the canonical Go/toolchain/dependency source.

```sh
cd system
go mod download
go test -shuffle=on ./...
```

For lint/generation, use the same commands documented in `../AGENTS.md` and CI. Firestore behavior that depends on Emulator semantics is checked from the repository root:

```sh
bash .github/scripts/run-firestore-integration-tests.sh
```

These checks are intentionally separate from real-service execution.

## Real-service execution

`go run ./cmd/youtube-bot` is **not a normal test command**. Startup loads local environment/credential configuration, connects to configured Google/YouTube/Discord services, and can post chat/notifications or mutate Firestore state.

Only run it for an explicitly authorized real-environment smoke test. Before proceeding past startup, verify the Google Cloud Project ID printed by the program is the intended target. Never copy credential values into documentation, issues, or logs.

## Local operator Google WIF preflight

`cmd/google-auth-preflight` is a read-only canary for local operator authentication through AWS IAM Identity Center, a dedicated AWS operator role, and Google Workload Identity Federation. It does not load `.env`, post to YouTube/Discord, or mutate Firestore; after authentication it only reads the system constants document.

The command binds each environment to both a known Google Cloud project and a dedicated AWS shared-config profile:

- development: `test-youtube-study-space` + `soraride-google-operator-dev`
- production: `youtube-study-space` + `soraride-google-operator-prod`

Those dedicated profiles must assume the environment's narrow Google-operator role from the existing IAM Identity Center source profile. Do not point them at the broad source role itself.

```ini
[profile soraride-google-operator-dev]
source_profile = soraride-dev
role_arn = <development-dedicated-google-operator-role-arn>
region = ap-northeast-1

[profile soraride-google-operator-prod]
source_profile = soraride-prod
role_arn = <production-dedicated-google-operator-role-arn>
region = ap-northeast-1
```

Log in to the source IAM Identity Center profile, then provide the non-secret WIF settings for the selected environment. Static AWS credential environment variables are rejected so they cannot override the dedicated profile.

```bash
cd system

aws sso login --profile soraride-dev
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN

export GOOGLE_CLOUD_PROJECT=test-youtube-study-space
export GCP_WIF_AUDIENCE='<development-wif-provider-audience>'
export GCP_WIF_SERVICE_ACCOUNT_EMAIL='<development-service-account-email>'

go run ./cmd/google-auth-preflight \
  development test-youtube-study-space
```

The `development` / `production` argument is bound to its known project and dedicated profile before credential loading. A mismatched environment/project pair, or a mismatch between the explicit project and `GOOGLE_CLOUD_PROJECT`, fails before AWS credentials are loaded. Google WIF must trust the dedicated operator role, not the broad IAM Identity Center source role.

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
- 生成コードは `core/i18n/internal/engine` を使用します（`engine.TranslateDefault(...)`）。
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


## youtube-bot のFargate移行

`cmd/youtube-bot` は移行期間中、2つのGoogle Cloud認証経路を持つ。

- 既定値 / `YOUTUBE_BOT_AUTH_MODE=service-account`: 現在の配信PC互換経路。 `.env` と `CREDENTIAL_FILE_LOCATION` を読み、従来どおりproject IDの対話確認を行う。
- `YOUTUBE_BOT_AUTH_MODE=wif`: ECS/Fargate用。 `.env` やService Account JSONを読まず、Task RoleからGoogle WIFを利用する。`YOUTUBE_BOT_ENVIRONMENT` と `GOOGLE_CLOUD_PROJECT` の組み合わせが既知のdevelopment/production対象と一致しない場合はAWS credential取得前に停止する。

Fargate向けにはside-effect-freeなpreflightを用意している。

```bash
/app/batch preflight
```

Fargate imageは既存の `Dockerfile.fargate` を `BUILD_TARGET=./cmd/youtube-bot` でビルドするため、container内の実行ファイル名は移行中も `/app/batch` のまま。preflightはFirestoreのcredentials設定、system constants、menu docsとNGワード用Google Sheetsをread-onlyで確認し、YouTube/Discordへの投稿やFirestore更新は行わない。既存の対話起動が警告対象にしているzero-value constantsもJSONへ列挙するが、`false` / `0` が正当な設定もあるため一律エラーにはしない。

ECS Serviceは初期状態でdesired count 0とし、Task RoleのGoogle WIF trustとpreflightが完了するまで常駐起動しない。実切替では配信PCの旧Botを停止してからServiceを1へ上げ、同じライブチャットを2つのBotが同時処理しないことを優先する。

## 日次バッチ（ECS Fargate）と通知（SNS→Lambda→Discord）

- 実行基盤: AWS ECS Fargate (arm64) 上の単一バッチコンテナ
- オーケストレーション: AWS Step Functions（直列実行）
- スケジュール: EventBridge Scheduler が **毎日 00:00 JST**（CDK では UTC 15:00）に `start_daily_batch` Lambda を実行し、Step Functions が起動。**SFN 定義では先頭に 15 秒の Wait（日付境界ずれ対策）**のあと ECS タスクが実行される
- 実行順序（ECS 上のジョブ）: `reset-daily-total` → `update-rp` → `transfer-bq`
- Google Cloud認証: AWS Task RoleからWorkload Identity Federation (WIF)で既存Service Accountをimpersonate
- ネットワーク: Public Subnet, Public IP割当, HTTPS/DNS/ECS Task credential endpointへの最小egress
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
  - `Base Image Update Report` workflow が、旧/新 digest、現在タグの digest、一致判定を同じ PR コメントへ自動で投稿・更新する
  - workflow はすべての PR `opened` / `synchronize` / `reopened` を拾い、最終差分から Dockerfile 更新が消えた場合は古い report comment を削除する。Dockerfile 更新がない場合は registry 照合を行わず終了する
  - Amazon ECR Public のイメージでは、対応する ECR Public Gallery へのリンクもコメントに表示する
  - `gcr.io` のイメージでは、対応する Google Artifact Registry のイメージ画面へのリンクもコメントに表示する
  - registry の現在タグと pinned digest を検証できない場合はコメント／Actions warningで明示するが、この report workflow 自体は advisory としてマージをブロックしない
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
- **Go の minor / major を上げる場合**は、`go.mod` の `go x.yy` と Dockerfile の `golang:x.yy@sha256:...` のタグを同一 minor に揃えること。この「base image 更新運用」を `system/` における詳細手順の正本とする。
