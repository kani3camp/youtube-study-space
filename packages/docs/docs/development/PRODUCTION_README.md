# 本番環境でのシステム構成

## 準備

### Firestore

GCPで`studywithocemeco@appspot.gserviceaccount.com`というサービスアカウントのKEYをjsonファイルとして発行し、Botプログラムが動くPC上に保存する。

### Lambda関数のデプロイ

1. `system/aws-lambda/deploy_production.sh`を手順通りに進める（必ずリージョンを確認）
   - デプロイするコードのgoファイル名を必要な部分に指定
   - ファイルのコンパイル・圧縮
   - aws cliでデプロイ

### API Gateway

1. AWSリージョンが本番用であることを確認
2. REST APIを新規作成
3. エンドポイントを作成し、各々lambda関数と統合
4. 少なくとも`/set_desired_max_seats`はAPIキーを設定
5. 作業が終わったら、最後にAPIをデプロイ

### Cloud Scheduler

1. GCPプロジェクトが本番用であることを確認
2. README.mdの情報を参照しながらよしなに設定

### DynamoDB

1. AWSリージョンが本番用であることを確認
2. README.mdの情報を参照しながらよしなに設定

### Cloud Functions

1. GCPプロジェクトが本番用であることを確認
2. README.mdの情報を参照しながらよしなに設定

## 実行手順

### Botプログラム

1. 本番用GCPのcredentialを使うように環境変数`CREDENTIAL_FILE_LOCATION`を設定する

### monitor

`api_config.ts`で

```ts
const api = prodApi
```

とする
