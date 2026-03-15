# YouTube Study Space - AI協業ガイド

## プロジェクト概要

YouTube Study Space は、YouTube ライブ配信のチャットコマンドを使って入退室や作業管理を行うオンライン自習室です。`system/` は、そのバックエンド、定期実行ジョブ、Lambda ハンドラをまとめた Go ベースの実装です。

### 現在のステータス
- 稼働中のプロダクトであり、Firestore を中心に状態を管理している
- ローカル実行用のライブチャット bot は `system/main.go`
- 日次バッチ本体は `system/cmd/batch/`
- 定期実行 Lambda は `system/aws-lambda/`

## 技術スタック

### 言語と主要依存
- Go 1.24.0
- Firestore
- BigQuery
- Cloud Storage
- YouTube Data API v3
- AWS Lambda
- AWS Step Functions
- AWS ECS Fargate
- AWS EventBridge / EventBridge Scheduler
- DynamoDB
- `go.uber.org/mock`

### 開発環境
- ローカル開発: Go, Docker, `.env`
- 本番運用: AWS Lambda, Step Functions, ECS Fargate, Google Cloud

## コードベース構造

このガイドは `system/` ディレクトリを前提に読む。

### 主要ディレクトリ
- `main.go`
  - ローカルでライブチャット bot を起動するエントリーポイント
- `core/workspaceapp/`
  - コマンド処理、席管理、休憩、バリデーション、日次バッチ処理の中核
- `core/repository/`
  - Firestore とのデータアクセス層
- `core/youtubebot/`
  - YouTube Live Chat API との連携
- `core/guardians/`
  - ライブ状態の監視やガード処理
- `core/moderatorbot/`
  - Discord 通知やモデレーション関連
- `core/mybigquery/`
  - BigQuery 連携
- `core/i18n/`
  - ロケール定義と型付き翻訳ラッパー
- `cmd/batch/`
  - Fargate 上で動く日次バッチ本体
- `aws-lambda/`
  - 定期実行・補助処理の Lambda ハンドラ
- `internal/logging/`
  - ロガー初期化
- `Dockerfile.lambda`
  - Lambda 群のコンテナビルド定義
- `Dockerfile.fargate`
  - Fargate 用日次バッチのコンテナビルド定義

### 主要ファイル
- `main.go`
  - `Bot` と `CheckLongTimeSitting` を起動し、ライブチャットをポーリングする
- `core/workspaceapp/workspace_app.go`
  - `WorkspaceApp` の初期化と主要依存の束ね込み
- `core/workspaceapp/command_seat.go`
  - 入室、退室、休憩、延長、注文など席関連コマンド
- `core/workspaceapp/command_user.go`
  - `!my`、`!rank`、`!info` などのユーザー系コマンド
- `core/workspaceapp/command_moderation.go`
  - `!kick`、`!block`、`!report` などモデレーション系コマンド
- `core/repository/interface.go`
  - `Repository` / `DBClient` のインターフェース
- `core/i18n/generate.go`
  - 型付き翻訳コード生成の `go:generate` エントリ
- `cmd/batch/main.go`
  - 日次バッチのジョブ切り替えと実行順管理

## 処理フロー

### ライブチャット処理
1. YouTube Live Chat API からメッセージを取得
2. コマンドを解析して `workspaceapp` に渡す
3. Firestore トランザクションを使って状態を更新する
4. 必要に応じて YouTube Live Chat や Discord に返信する

### 定期実行
- **1 分ごと**
  - `youtube_organize_database`
  - `check_live_stream_status`
- **15 分ごと**
  - `update_work_name_trend`
- **毎日 00:00 JST**
  - EventBridge Scheduler が `start_daily_batch` Lambda を起動
  - `start_daily_batch` が Step Functions を開始
  - Step Functions が ECS Fargate 上の `cmd/batch` を実行

### 日次バッチの主な役割
- 日次学習時間のリセット
- RP 更新
- Firestore / GCS から BigQuery への履歴転送

## データモデル

### 主要エンティティ
- `SeatDoc`
  - 席情報、ユーザー ID、入室時刻、作業内容など
- `UserDoc`
  - ユーザー情報、累計時間、各種設定など
- `ConstantsConfigDoc`
  - 最大席数やポーリング間隔などのシステム定数
- `CredentialsConfigDoc`
  - 認証や外部接続に関する設定参照

### 状態管理
- Firestore を使って席状態とユーザー状態を永続化する
- 席関連更新はトランザクションで整合性を保つ

## 開発規約と協業ルール

### コーディング規約
- Go 標準のコーディング規約に従う
- エラーメッセージには文脈情報を含める
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
- 実行したコマンドや確認内容は会話上で明示する
- 機密値は表示せず、必要なら取得方法のみ案内する

## よく使う開発コマンド

`system/` で実行する。

```bash
# ライブチャット bot を起動
go run main.go

# テスト実行
go test -shuffle=on -v ./...

# 特定パッケージだけ実行
go test ./core/youtubebot/...

# Repository モック生成
mockgen -source ./core/repository/interface.go -destination ./core/repository/mocks/interface.go -package mock_repository

# 型付き i18n ラッパー再生成
go generate ./...

# 依存整理
go mod tidy
```

## コマンド一覧

実装上の主なチャットコマンド:

- `!in` - 一般席に入室
- `/in` - メンバー席に入室
- `!out` - 退室
- `!break` / `!rest` / `!chill` - 休憩開始
- `!resume` - 休憩終了
- `!my` - 自分の情報表示や設定変更
- `!rank` - ランキング表示
- `!info` - ユーザー情報表示
- `!seat` - 席情報表示
- `!change` - 作業内容変更
- `!more` / `!okawari` - 作業時間延長
- `!report` - 報告
- `!order` - 注文関連
- `!check` - 席状態確認
- `!kick` - キック
- `!block` - ブロック

## 参考

- [Go言語公式ドキュメント](https://go.dev/doc/)
- [YouTube Data API ドキュメント](https://developers.google.com/youtube/v3)
- [Google Cloud Firestore ドキュメント](https://cloud.google.com/firestore/docs)
- [AWS Lambda ドキュメント](https://docs.aws.amazon.com/lambda/)
