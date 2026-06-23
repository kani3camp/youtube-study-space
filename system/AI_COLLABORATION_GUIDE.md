# YouTube Study Space - AI協業ガイド

## プロジェクト概要

YouTube Study Space は、YouTube ライブのチャットコマンドで入退室や作業管理を行うオンライン自習室です。
`system/` はそのバックエンド、定期ジョブ、Lambda ハンドラをまとめた Go 実装です。
Firestore を中心に状態を保持し、[`cmd/youtube-bot/`](cmd/youtube-bot/) がローカル用ライブチャット bot、[`cmd/batch/`](cmd/batch/) が Fargate 日次バッチ、[`cmd/lambda/`](cmd/lambda/) が定期 Lambda です。

## 技術スタック

### 言語と主要依存
- Go 1.25.0
- Firestore
- BigQuery
- Cloud Storage
- YouTube Data API v3
- Discord（discordgo）
- OpenAI API（作業名トレンド `update_work_name_trend` など）
- AWS Lambda
- AWS Step Functions
- AWS ECS Fargate
- AWS EventBridge / EventBridge Scheduler
- DynamoDB
- `go.uber.org/mock`

### 開発環境
- ローカル: Go, Docker, `.env`
- 本番: AWS Lambda / Step Functions / Fargate, Google Cloud

## コードベース構造

このガイドは `system/` を前提に読む。

### 主要ディレクトリと参照しやすいファイル
- `cmd/youtube-bot/` — ローカル用エントリ。`Bot` と `CheckLongTimeSitting` を起動し、ライブチャットをポーリングする
- `core/workspaceapp/` — コマンド処理・席管理・休憩・バリデーション・日次バッチ周りの中核。`workspace_app.go` と `command_*.go`。日次・席数調整・作業名トレンドは `batch.go`・`max_seats_adjustment.go`・`work_name_trend.go` など。`presenter/`・`usecase/` で層分割
- `core/repository/` — Firestore。インターフェースは [`interface.go`](core/repository/interface.go)
- `core/youtubebot/` — YouTube Live Chat API
- `core/guardians/` — ライブ監視・ガード
- `core/moderatorbot/` — Discord・モデレーション
- `core/mybigquery/` — BigQuery
- `core/i18n/` — ロケールと型付きラッパー。[`generate.go`](core/i18n/generate.go) は `//go:generate go run app.modules/cmd/i18n-gen` の薄いエントリ（実体は [`cmd/i18n-gen/`](cmd/i18n-gen/)）
- `cmd/batch/` — Fargate 日次バッチ。[`main.go`](cmd/batch/main.go) でジョブ切り替え
- `cmd/lambda/` — 定期・補助 Lambda
- `internal/awsruntime/` — Lambda / Fargate など AWS 実行基盤向けの共通処理
- `internal/adminops/` — 管理者向け直接操作
- `internal/logging/` — ロガー初期化
- `Dockerfile.lambda` / `Dockerfile.fargate` — それぞれ Lambda 群・日次バッチ用コンテナ

## 処理フロー

### ライブチャット処理
1. YouTube Live Chat API からメッセージを取得
2. コマンドを解析して `workspaceapp` に渡す
3. Firestore トランザクションで状態を更新
4. 必要に応じて YouTube Live Chat や Discord に返信

### 定期実行
- **1 分ごと**
  - `youtube_organize_database`
  - `check_live_stream_status`
- **15 分ごと**
  - `update_work_name_trend`
- **毎日 00:00 JST**
  - EventBridge Scheduler が `start_daily_batch` Lambda を起動
  - `start_daily_batch` が Step Functions を開始し、**定義済みの 15 秒 Wait（日付境界ずれ対策）**の後に ECS Fargate 上で日次ジョブを直列実行（`cmd/batch` コンテナ、`reset-daily-total` → `update-rp` → `transfer-bq`）

### 日次バッチの主な役割
- 日次学習時間のリセット
- RP 更新
- Firestore / GCS から BigQuery への履歴転送

## データモデル

### 主要エンティティ
- `SeatDoc` — 席、ユーザー ID、入室時刻、作業内容など
- `UserDoc` — ユーザー、累計時間、設定など
- `ConstantsConfigDoc` — 最大席数・ポーリング間隔など
- `CredentialsConfigDoc` — 認証・外部接続の参照

### 状態管理
- Firestore で席・ユーザーを永続化し、席まわりの更新はトランザクションで整合性を保つ

## 開発規約と協業ルール

### コーディング規約
- Go 標準のコーディング規約に従う
- エラーメッセージには文脈を含める
- 単一責任を意識して関数を分割する
- `NOTE` コメントは重要な意図を含むため削除しない
- 一時的な補足コメントは `[NOTE FOR REVIEW]` を使う

### テスト方針
- ユニットテストは `*_test.go` で実装する
- モック生成は `go.uber.org/mock` を使う
- 主要なコマンド解析、Firestore リポジトリ、`workspaceapp` の振る舞いを優先して検証する

### AI協業の運用ルール
- 差分提案や非破壊な確認コマンドは許可不要
- コミット、プッシュ、PR 作成、デプロイ、破壊的操作は事前確認を取る
- 通常の開発用 PR はベースを **`dev`** にする（`gh pr create` では **`--base dev`**）。`main` は本番相当のため向けない
- 実行したコマンドや確認内容は会話上で明示する
- 機密値は表示せず、必要なら取得方法のみ案内する

### Go toolchain とベースイメージのバージョン整合
- `system/go.mod` の `go x.yy` と `system/Dockerfile.lambda` / `system/Dockerfile.fargate` の `FROM golang:x.yy@sha256:...` は **常に同じ minor バージョンに揃える**
- Go の minor / major を上げる場合は、以下を 1 つの PR にまとめる
  - `system/go.mod` の `go x.yy` 更新
  - `Dockerfile.lambda` / `Dockerfile.fargate` の `FROM golang:x.yy@sha256:...` のタグ更新（digest も新タグのものに差し替え）
  - digest は `docker buildx imagetools inspect docker.io/library/golang:x.yy --format '{{.Manifest.Digest}}'` で取得する
- patch レベル（同一 minor 内での digest 変化）は Dependabot の docker ecosystem が自動で PR 化するため、人手の調整は不要

## よく使う開発コマンド

`system/` で実行する。

```bash
# ライブチャット bot を起動
go run ./cmd/youtube-bot

# テスト実行
go test -shuffle=on -v ./...

# 特定パッケージだけ実行
go test ./core/youtubebot/...

# Repository モック生成
mockgen -source ./core/repository/interface.go -destination ./core/repository/mocks/interface.go -package mock_repository

# 型付き i18n（generate.go → cmd/i18n-gen）
go generate ./...

# 依存整理
go mod tidy
```

## コマンド一覧

**網羅的なコマンド文字列（`!` / `/` や別名）は [`core/utils/constants.go`](core/utils/constants.go) を正とする。**

代表例:

- `!in` / `/in` — 一般席 / メンバー席入室
- `!out` — 退室
- `!break` / `!rest` / `!chill` — 休憩、`!resume` — 再開
- `!my` — 自分の情報・設定、`!rank` — ランキング
- `!more` / `!okawari` — 作業時間延長
- `!order` — 注文関連（例: 下膳 `!order -`）
- モデレーション: `!kick`, `!check`, `!block`（メンバー側に `/kick` など別定義あり）

## 参考

- [Go言語公式ドキュメント](https://go.dev/doc/)
- [YouTube Data API ドキュメント](https://developers.google.com/youtube/v3)
- [Google Cloud Firestore ドキュメント](https://cloud.google.com/firestore/docs)
- [AWS Lambda ドキュメント](https://docs.aws.amazon.com/lambda/)
