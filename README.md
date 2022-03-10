# youtube-study-space
Youtubeで24時間365日ライブ配信し続ける、オンライン自習室！
視聴者はライブチャットからコマンドを打つことで自由に入退室できます。

[Youtubeチャンネルへ](https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg)



# テスト環境と本番環境
## 環境分けについて
### GCP
- 本番環境：`youtube-study-space`
- テスト環境：`test-youtube-study-space`

### AWS
- 本番環境：Tokyo (ap-northeast-1) リージョン
- テスト環境：N. Virginia (us-east-1) リージョン


## 手順書
- テスト環境：`TEST_README.md`
- 本番環境：`PRODUCTION_README.md`


# 共通設定
## Firestore
### データ
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
#### 環境変数：なし

### rooms_state
monitorからAPIで呼ばれる。
ルームの状況および最大席数などの情報を返す。
#### 環境変数：なし

### reset_daily_total_study_time
cloud schedulerにより**毎日0時0分**に実行される。
全ユーザーのデイリー作業時間を0にリセットする。
#### 環境変数：なし

### check_live_stream_status
cloud schedulerにより**毎分**実行される。
ライブ配信の状態がactiveであるかどうかチェックする。
activeでない場合はLINEで通知する。
#### 環境変数：なし

### set_desired_max_seats
monitorにより必要な時にAPIで呼ばれる。
monitor側で席数を変更すべきと判断したときの、希望の席数をfirestoreに保存する。
#### 環境変数：なし


## DynamoDB
Lambda関数と同じregionのDyanamoDBテーブルであること！
### データ
- Firestoreのアクセス情報（サービスアカウント）のjson文字列

注意：json内で出てくるprivate keyの値の文字列内のエスケープは調整する必要があった気がする。

### テーブル名：`secrets`


## API Gateway
### API名：`youtube-study-space-rest-api`
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
### Topic name: `projects/youtube-study-space/topics/initiateFirestoreExport`


## Cloud Functions
### firestoreExport
cloud scheduler + Pub/Subにより**毎日**実行される。
Firestoreのデータをエクスポートする。
#### 環境変数：なし


## Youtubeモニター
- ローカルでNext.jsのサーバーを立てる。
- `public/audio/lofigirl/`に音声ファイルを入れておく。 
フォルダ構成は`youtube-monitor/lib/bgm.ts`の内容と合わせる。
- `youtube-monitor/.env`を作成し、環境変数`NEXT_PUBLIC_API_KEY`を設定しておく。
値はAPI Gatewayで設定したAPIキー。



## OBS Studio
### 設定など
- OBS内蔵のブラウザを使用する
- 「OBSを介して音声を制御する」を有効にする（じゃないとBGMが配信に流れない）
- リンクに、ローカルで起動しているyoutube-monitorのサーバーのアドレスを入力する
- **マイクは必ずOFFにする**
- **カメラをソースとして登録したシーンは絶対に登録しない。** OBSは配信専用として使う。
当たり前だけど、本番配信中にシーンいじるとそれが映像に反映される（一回やらかした）


## Youtube Live配信
すぐに配信を始めると、少し映像ストリームが途切れただけで勝手にライブ配信が終了してしまい無限に配信できないので、「SCHEDULE STREAM」から配信予定を立てて、配信を開始する。
こうすることで途中何かのトラブルである程度の時間OBSからの映像送信が途切れてもライブ配信が勝手に終了することはない。
