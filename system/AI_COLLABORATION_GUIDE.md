# Youtube Study Space - AI協業ガイド

## プロジェクト概要

Youtube Study Spaceは、YouTubeのライブ配信を活用したオンライン学習・作業スペースを提供するサービスです。ユーザーはYouTubeのライブチャットを通じてコマンドを送信し、バーチャルな「席」を確保して学習や作業を行うことができます。

### 主要機能
- ユーザーがライブチャットで席の予約・退席などのコマンドを送信
- 一般席とメンバー限定席の管理
- 作業時間の記録と統計
- 長時間席を占有しているユーザーの検出
- モデレーター機能（キック、ブロックなど）
- 休憩機能

### ターゲットユーザー
- オンラインで集中して学習・作業したいユーザー
- YouTubeのライブ配信を視聴しながら学習コミュニティに参加したい人
- チャンネルのメンバーシップ会員（特別席の利用が可能）

### 現在のステータス
本システムは稼働中のプロダクトで、AWS Lambdaを含むクラウドサービス上で運用されています。プロジェクトは継続的に機能追加やバグ修正が行われています。
また、system/main.goはローカルで実行するライブチャットbotプログラムです。

## 技術スタック

### プログラミング言語
- Go言語 (v1.23)

### 主要ライブラリ/フレームワーク
- Google Cloud Firestore: データベース
- Google Cloud BigQuery: 分析用データストア
- Google Cloud Storage: ファイルストレージ
- YouTube API v3: YouTubeライブチャット連携
- AWS Lambda: サーバーレス関数
- AWS SDK: AWSサービス連携

### 開発環境
- ローカル開発環境: Go, Docker
- 本番環境: AWS Lambda, Google Cloud

### 外部サービス連携
- YouTube Data API
- Firestore
- BigQuery
- Discord (通知用)

## コードベース構造

### ディレクトリ構造
- `/` - プロジェクトルート
  - `main.go` - アプリケーションのエントリーポイント
  - `core/` - システムのコアロジック
    - `system.go` - メインのシステム機能
    - `youtubebot/` - YouTubeライブチャット連携
    - `repository/` - データストアへのアクセス層
    - `utils/` - ユーティリティ関数
    - `i18n/` - 国際化対応
    - `guardians/` - セキュリティ関連機能
    - `moderatorbot/` - モデレーター機能
    - `mybigquery/` - BigQuery連携
    - `mystorage/` - ストレージ連携
  - `direct-operations/` - 直接操作用のツール
  - `aws-lambda/` - AWS Lambda関数
  - `Dockerfile` - コンテナ化定義

### 主要ファイル
- `main.go`: アプリケーションのエントリーポイント。`Bot`関数と`CheckLongTimeSitting`関数を起動
- `core/system.go`: システムの中核機能。コマンド処理、席管理などの主要ロジックを含む
- `core/type_system.go`: システムの型定義
- `core/repository/firestore_controller_interface.go`: Firestoreとのインターフェース

## データモデルと処理フロー

### 主要データモデル
- `SeatDoc`: 席の情報（ユーザーID、入室時間、作業内容など）
- `UserDoc`: ユーザー情報（累計学習時間、プロフィールなど）
- `ConstantsConfigDoc`: システム定数（最大席数、ポーリング間隔など）
- `CredentialsConfigDoc`: 認証情報

### 主要処理フロー
1. YouTubeライブチャットからメッセージを取得
2. コマンドを解析して適切な処理を実行
3. Firestoreのデータを更新
4. 結果をYouTubeライブチャットに返信

### 状態管理
- Firestoreを使用してユーザーの状態、席の状態を永続化
- トランザクション処理によるデータ整合性の確保

## 開発規約とガイドライン

### コーディング規約
- Go標準のコーディング規約に準拠
- エラーハンドリングは適切に行い、エラーメッセージには文脈情報を含める
- 関数は単一責任の原則に従って設計

### 命名規則
- 変数・関数名: キャメルケース（`myVariable`, `MyFunction`）
- パッケージ名: 小文字のみ（`repository`, `utils`）
- 定数: 大文字スネークケース（`MAX_RETRY_ATTEMPTS`）

### テスト戦略
- ユニットテスト: `*_test.go`ファイルで実装
- モックを使用したテスト: `go.uber.org/mock`を使用

## テストとQA

### テストフレームワーク
- Go標準のテストパッケージ
- テストモック: `go.uber.org/mock`

### テストカバレッジ
- 主要なビジネスロジックに対してユニットテストを実施
- モックを使用して外部依存（Firestore、YouTube API）を置き換えてテスト

## 既知の課題と制約

### パフォーマンス
- YouTubeライブチャットAPIのクォータ制限
- 長時間実行時のメモリ使用量

### セキュリティ
- 認証情報（`.env`ファイル、Googleサービスアカウント）の適切な管理
- ユーザー入力のバリデーション

## よくある開発タスク

### 環境設定
1. リポジトリをクローン
2. `.env`ファイルを設定
3. 必要なCredentialsファイルを配置
4. `go mod download`で依存関係をインストール

### ローカル開発
```bash
# 開発環境の起動
go run main.go

# テストの実行
go test ./...

# モックの生成
mockgen -source=core/repository/firestore_controller_interface.go -destination=core/repository/mocks/firestore_controller_interface.go -package=mock_myfirestore
```

### デプロイ
デプロイには AWS CDK を使用します。

デプロイの詳細な手順については、プロジェクトのルートディレクトリの親ディレクトリにある `aws-cdk` ディレクトリの README.md を参照してください。AWS CDKを使用することで、インフラストラクチャをコードとして管理しています。

## 用語集

| 用語          | 説明                                                    |
| ------------- | ------------------------------------------------------- |
| 席（Seat）    | ユーザーが作業するための仮想的なスペース                |
| 一般席        | 誰でも利用可能な席                                      |
| メンバー席    | YouTubeチャンネルのメンバーシップ会員のみが利用可能な席 |
| 入室（In）    | 席を確保すること                                        |
| 退室（Out）   | 席を解放すること                                        |
| 休憩（Break） | 一時的に作業を中断すること                              |
| RP            | リワードポイント（報酬ポイント）システム                |

## コマンド一覧

システムで使用できる主なコマンド：

- `!in` - 入室（一般席）
- `/in` - 入室（メンバー席）
- `!out` - 退室
- `!break` または `!rest` または `!chill` - 休憩開始
- `!resume` - 休憩終了
- `!my` - 自分の情報表示
- `!rank` - ランキング表示
- `!check` - 席の状態確認
- `!info` - ユーザー情報表示
- `!seat` - 席情報表示
- `!change` - 作業内容変更
- `!more` または `!okawari` - 作業時間延長
- `!report` - 報告
- `!order` - 注文（メンバー機能）
- `!kick` - キック（モデレーター機能）
- `!block` - ブロック（モデレーター機能）

## 参考リソース

- [Go言語公式ドキュメント](https://golang.org/doc/)
- [YouTube Data API ドキュメント](https://developers.google.com/youtube/v3)
- [Google Cloud Firestore ドキュメント](https://cloud.google.com/firestore/docs)
- [AWS Lambda ドキュメント](https://docs.aws.amazon.com/lambda/) 
