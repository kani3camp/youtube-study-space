# Study w/ 王攻子

[youtube-study-space](https://github.com/sorarideblog/youtube-study-space)のForkです。

## 主な変更点

### 全体

- プロジェクト名を`studywithocemeco`に変更
- ドキュメントをDocusaurus化
- `commitlint`の導入
- バージョンを`standard-version`で管理

### バックエンド

- 部屋数の自動調整を削除(予定)
- LINE通知を廃止(予定)

### フロントエンド

- プレビューをThree.jsで3D化(予定)
- 部屋ごとにキャラクターを配置(予定)

### 配信

- VPS上での24時間配信に変更(予定)

---

以下、元のREADMEを環境に合わせて編集しています。

## 環境分けについて

### GCP

- 本番環境：`studywithocemeco`
- テスト環境：`test-studywithocemeco`

### AWS

- 本番環境：Tokyo (ap-northeast-1) リージョン
- テスト環境：N. Virginia (us-east-1) リージョン

## 手順書

- テスト環境：`TEST_README.md`
- 本番環境：`PRODUCTION_README.md`

## Firestore

- ルームの入室状況
- ライブ配信用youtubeチャンネルのAPIアクセス情報
- Bot用youtubeチャンネルのAPIアクセス情報
- ラインBotのアクセス情報
- youtubeライブ配信の情報
- オンライン作業部屋のユーザー情報
  - 入退室ログ
- システムconfig
  - デフォルト入室時間
  - 設定可能な最大入室時間
  - 設定可能な最小入室時間
  - 席数
  - その他

### データ構造

`system/core/myfirestore/type_firestore_data.go`を参照。

## Lambda関数

### youtube_organize_database

cloud schedulerにより**毎分**実行される。

入室中のユーザーから，自動退室予定時刻を過ぎているユーザーを発見して，退室処理をする。

環境変数：なし

### rooms_state

monitorからAPIで呼ばれる。

ルームの状況および最大席数などの情報を返す。

環境変数：なし

### reset_daily_total_study_time

cloud schedulerにより**毎日0時0分**に実行される。

全ユーザーのデイリー作業時間を0にリセットする。

環境変数：なし

### check_live_stream_status

cloud schedulerにより**毎分**実行される。

ライブ配信の状態がactiveであるかどうかチェックする。activeでない場合はLINEで通知する。

環境変数：なし

### set_desired_max_seats

monitorにより必要な時にAPIで呼ばれる。

monitor側で席数を変更すべきと判断したときの、希望の席数をfirestoreに保存する。

環境変数：なし

## DynamoDB

Lambda関数と同じregionのDyanamoDBテーブルであること！

### データ

- Firestoreのアクセス情報（サービスアカウント）のjson文字列

注意：json内で出てくるprivate keyの値の文字列内のエスケープは調整する必要があった気がする。

テーブル名：`secrets`

## API Gateway

### API名：`studywithocemeco-rest-api`

#### エンドポイント

各エンドポイントは、同じ名前のlambda関数と統合する。

- GET /organize_database
- GET /reset_daily_total_study_time
- GET /rooms_state
- GET /check_live_stream_status
- POST /set_desired_max_seats

## Cloud Scheduler

- call_lambda_function_organize_database: `* * * * *`
- call_lambda_function_reset_daily_total_study_time: `0 0 * * *`
- scheduledFirestoreExport: `every 24 hours (Asia/Tokyo)`
- call_lambda_function_check_live_stream_status: `* * * * *`

## Pub/Sub

[このドキュメント](https://firebase.google.com/docs/firestore/solutions/schedule-export) を参考に、毎日1回firestoreのデータをcloud storageにバックアップする。

Topic name: `projects/studywithocemeco/topics/initiateFirestoreExport`

## Cloud Functions

### firestoreExport

cloud scheduler + Pub/Subにより**毎日**実行される。
Firestoreのデータをエクスポートする。

環境変数：なし

## Youtubeモニター

- ローカルでNext.jsのサーバーを立てる。
- `public/audio/lofigirl/`に音声ファイルを入れておく。 
フォルダ構成は`youtube-monitor/lib/bgm.ts`の内容と合わせる。
- `youtube-monitor/.env`を作成し、環境変数`NEXT_PUBLIC_API_KEY`を設定しておく。
値はAPI Gatewayで設定したAPIキー。
