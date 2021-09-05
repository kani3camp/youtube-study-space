# youtube-study-space
Youtubeで24時間365日ライブ配信し続ける、オンライン自習室！
視聴者はライブチャットからコマンドを打つことで自由に入退室できます。

[Youtubeチャンネル](https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg)



# テスト環境と本番環境
## 環境分けについて
### GCP
- 本番環境：`youtube-study-space`
- テスト環境：`test-youtube-study-space`

### AWS
- 本番環境：Tokyo (ap-northeast-1) リージョン
- テスト環境：Osaka (ap-northeast-3) リージョン


## 手順書
- テスト環境：`TEST_README.md`
- 本番環境：`PRODUCTION_README.md`


# 共通設定
## Firestore
### データ
- ルームに関する情報
- 各種youtubeチャンネルのAPIアクセス情報
- ラインBotのAPIアクセス情報
- ユーザー情報
  - 入退室ログ
- システムconfig
### データ構造
- configコレクション
  - constants
  - default-rom-layout
    - historyコレクション
    - レイアウトデータ
  - line-bot
  - youtube-bot-credential
  - youtube-channel-credential
  - youtube-live
- roomsコレクション
  - default
    - seats
  - no-seat
    - seats
- usersコレクション
  - youtubeのチャンネルID
    - historyコレクション
    - その他設定



## Lambda関数
### youtube_organize_database
cloud schedulerにより**毎分**実行される。
自動退室処理をする。
#### 環境変数：なし

### rooms_state
monitorからAPIで呼ばれる。
デフォルトルームとスタンディングルームの状況およびデフォルトルームのレイアウトを返す。
#### 環境変数：なし

### reset_daily_total_study_time
cloud schedulerにより**毎日0時0分**に実行される。
全ユーザーのデイリー作業時間を0にリセットする。
#### 環境変数：なし


## DynamoDB
### データ
- Firestoreのアクセス情報（サービスアカウント）
### テーブル名：`secrets`


## API Gateway
### API名：`youtube-study-space-http-api`
#### エンドポイント
- /organize_database
- /reset_daily_total_study_time
- /rooms_state



## Cloud Scheduler
- call_lambda_function_organize_database: `* * * * *`
- call_lambda_function_reset_daily_total_study_time: `0 0 * * *`
- scheduledFirestoreExport: `every 24 hours (Asia/Tokyo)`


## Pub/Sub
### Topic name: `projects/youtube-study-space/topics/initiateFirestoreExport`


## Cloud Functions
### firestoreExport
cloud scheduler + Pub/Subにより**毎日**実行される。
Firestoreのデータをエクスポートする。
#### 環境変数：


## Youtubeモニター
- ローカルでNext.jsのサーバーを立てる。
- WindowsはDockerを使うとよい。
- public/audio/lofigirl/に音声ファイルを入れておくこと。



## OBS Studio
### 設定など
- OBS内蔵のブラウザを使用する
- 「OBSを介して音声を制御する」を有効にする（じゃないとBGMが配信に流れない）
- リンクに、ローカルで起動しているyoutube-monitorのサーバーのアドレスを入力する
- マイクは必ずOFFにする


## Youtube Live配信
すぐに配信を始めると、無限に配信できないので、「SCHEDULE STREAM」から配信予定を立てて、配信を開始する。

