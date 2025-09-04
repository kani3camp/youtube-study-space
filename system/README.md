
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

### mockgenをインストールする
```shell
go install go.uber.org/mock/mockgen@latest
```

### mockgenのバージョン確認
```shell
mockgen --version
```

### systemディレクトリに移動する
```shell
cd system
```

### mockファイルを作成する
* Repositoryの場合
```shell
mockgen -source=core/repository/interface.go -destination=core/repository/mocks/interface.go -package=mock_repository
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
