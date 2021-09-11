# 本番環境でのシステムシステム構成（前準備）
## Firestore
GCPで`youtube-study-space@appspot.gserviceaccount.com`というサービスアカウントのKEYをjsonファイルとして発行し、Botプログラムが動くPC上に保存する。


## Lambda関数のデプロイ
1. `deploy_production.sh`を手順通りに進める（必ずリージョンを確認）
2. Lambdaコンソール上で関数がデプロイされたことを確認
3. アップデートしたLambda関数のバージョンとエイリアスを作成し、公開
   1. アップデートしたLambda関数のバージョンを公開
   2. エイリアス`stg`がなければ作成し、バージョン`$LATEST`を設定する。
   3. エイリアス`prod`がすでに作成されており、特定の安定したバージョンが設定されていることを確認する。
   4. `stg`バージョンで最新のLambda関数が正しく動作することを確認したら、`prod`バージョンに`stg`バージョンと同じバージョンを設定する。


## API Gateway
1. AWSリージョンが本番用であることを確認


## Cloud Scheduler
1. GCPプロジェクトが本番用であることを確認


## DynamoDB
1. AWSリージョンが本番用であることを確認


## Cloud Functions
1. GCPプロジェクトが本番用であることを確認


# 実行手順
## Botプログラム
1. 本番用GCPのcredentialを使うように環境変数を設定する


## monitor


