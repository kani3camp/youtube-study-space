# メタデータ削除の監視実装と接続手順

29 日経過した公開メタデータを削除する定期ジョブの呼び出しと、再起動を跨いで監視状態を保持する仕組みを実装しています。

> [!NOTE]
> 本ドキュメントはソースコード上の仕様と実行コマンドを説明しています。実環境での Cloud Scheduler・クラウドランタイム・IAM・通知先の設定は別途行う必要があります（[配備手順書](README.md) を参照）。

## 実行コマンド（`cmd/mypage-metadata-cleanup`）

[`system/cmd/mypage-metadata-cleanup`](../../../system/cmd/mypage-metadata-cleanup/main.go) が `MetadataCleanupRunner` を Firestore クライアントと Cloud Logging の健全性シグナル出力先に接続します。バイナリは Maintenance Operator と同じイメージ（[`system/Dockerfile.mypage-maintenance-operator`](../../../system/Dockerfile.mypage-maintenance-operator)）の `/app/mypage-metadata-cleanup` に含め、Cloud Run Job から command 指定で起動する想定です（イメージの ENTRYPOINT は Maintenance Operator のままです）。Cloud Run Job・Cloud Scheduler・IAM・アラートの設定はこのコマンドの外で別途行います。

- 実行モード: 引数は `-mode=run`（削除処理 `Run`）か `-mode=health`（独立監視 `CheckHealth`）のどちらか 1 つだけです。引数なし・未知の値・重複指定・余分な引数は、認証情報の探索や Firestore 接続より前に終了コード 1 で拒否します。
- 実行先の固定: `MYPAGE_ENVIRONMENT` は `development`（`test-youtube-study-space`、`asia-southeast2`）か `production`（`youtube-study-space`、`asia-northeast2`）のどちらかで、`GOOGLE_CLOUD_PROJECT` と一致する必要があります。ログのリソースラベルには Cloud Run Job が設定する `CLOUD_RUN_JOB` を使い、未設定や不正な名前の場合もクライアント作成前に終了します。
- 処理量と制限時間: 1 回の `Run` は 100 件 × 最大 100 ページを処理し、残件があれば成功扱いにしません。プロセス全体の制限時間は `run` が 11 分（削除処理本体の 10 分 + 後処理の余裕）、`health` が 30 秒です。
- 必要な権限: 実行サービスアカウントには、Firestore の `web-accounts` と `mypage-maintenance/metadata-cleanup` の読み書き（例: `roles/datastore.user`）と、健全性シグナル用の `logging.logEntries.create` が必要です。
- 出力: 標準出力には固定形の JSON を 1 行だけ書きます。`run` は `mode` と匿名の観測結果（開始・完了時刻、結果、固定段階名、件数）、`health` は `mode`・`checkedAt`・`alert`・`reason` です。失敗時は標準エラーへ固定の状態文字列だけを書いて終了コード 1 で終了し、依存先の生エラーは公開ログに出しません。

### 健全性シグナルの出力先（Cloud Logging）

`CloudLoggingMetadataCleanupHealthRecorder` は、シグナルごとに Cloud Logging の `entries:write`（`https://logging.googleapis.com/v2/entries:write`）へ構造化ログを 1 件書き込みます。標準出力は書き込み期限を設定できないため使いません。

- ログ名: `projects/<project>/logs/mypage-metadata-cleanup-health`
- リソース: `cloud_run_job`（ラベル `job_name`・`location`・`project_id`）
- 重大度: `alert` が真なら `WARNING`、偽なら `INFO`
- `jsonPayload`: `checkedAt`・`alert`・`reason` だけを含みます（許可リスト外の値や `alert` と `reason` の不整合は送信前に拒否します）。
- 認証: アプリケーションのデフォルト認証情報から `logging.write` スコープのトークンを取得します。実行サービスアカウントには `logging.logEntries.create` 権限が必要です。
- 受理の判定: ランナーが与える 2 秒の制限時間内に HTTP 2xx を受け取った場合だけ受理とみなします。それ以外のステータス・タイムアウト・トークン取得失敗はエラーとし、エラーには固定の文言とステータスコードだけを含めます（応答本文やトークンは含めません）。リダイレクトは追いません。

### 実行時間とスケジュールの設計

- タイムアウト予算
  - `Run`: 削除処理本体に 10 分の制限時間を適用します（29 日判定・リビジョン保護・残件検査を含む）。ジョブ終了後の監視状態保存には独立した 2 秒、健全性読み取り・シグナル書き込み・期限違反 ACK にも各 2 秒の制限を設けます。
  - スケジューラー側には「10 分 + 後処理の余裕」を持たせたタイムアウトを設定し、返却エラーを失敗として扱います（キャンセル時も失敗シグナルの記録を試みます）。
  - `CheckHealth`: アカウントの走査や削除を行わず、単独で呼び出せます。
- 起動間隔
  - 削除処理（`Run`）は、成功した走査の開始間隔が 23 時間未満になる頻度（例：1 日 2 回）で実行してください。
  - 独立監視（`CheckHealth`）は例として毎時起動し、削除ジョブ自体の起動漏れや停止を検知します（削除ジョブの終了時だけ確認する構成では、次回起動しない障害を検知できません）。
  - 独立監視自体の未起動や出力先障害を検知する仕組みは、スケジューラー・ランタイム側の失敗監視と組み合わせて確保します。
- シグナル出力先の要件
  - 実行コマンドは Cloud Logging の出力先（上記）を使います。
  - `JSONMetadataCleanupHealthRecorder` は、書き込み中の期限設定（`SetWriteDeadline`）やキャンセルが実際に機能する専用パイプやソケットを必要とします。通常ファイル・標準出力・無期限の `io.Writer` は安全のため拒否します。
  - 配送経路・通知先は承認済み一覧から選び、サンプル値を実環境へ流用しないでください。

---

## 監視状態とシグナルの仕様

### 保存データ（サーバー専用単一文書）

既存のセキュリティルールで既定拒否となる `mypage-maintenance/metadata-cleanup` に、単一文書として次の情報をトランザクションで保存します（新しいクエリインデックスは不要です）。

- 最新の匿名観測結果
- 最後に成功した走査の開始時刻（`lastSuccessfulCoverage`）
- 出力先に未受理の保持期限違反の開始時刻（`pendingRetentionStartedAt`）

※ユーザー ID・メタデータ・認証情報・生エラー・実行別の履歴一覧は保存しません。コンストラクターでは削除処理と監視が同一プロジェクトであることを確認します。

### 時刻比較と状態保護のルール

- 走査開始時刻基準: 完了時刻ではなく走査の開始時刻を使います（比較前に Firestore のマイクロ秒精度へ正規化します）。
- 古い実行の上書き防止: 遅延して到着した古い走査結果や再送結果で、より新しい失敗を上書きしません。開始時刻が同じ場合は失敗を優先します。
- 破損状態の保護: 監視状態が不正または破損している場合は正常値へ初期化せずエラーとし、運用調査を求めます。
- 未受理違反の保持と ACK
  - 後続の正常実行で最新観測が置き換わっても、出力先に受理されていない保持期限違反は保留（`pendingRetentionStartedAt`）に残します。
  - 出力先が受理した後にだけ、想定した走査開始時刻が一致する場合に限りトランザクションで ACK します（並行して記録された新しい違反は消しません）。
  - シグナル出力や ACK が失敗した場合は再通知可能なまま残し、監視ストアが利用可能なら監視失敗も記録します。

### 健全性シグナルの出力形式

出力には `checkedAt`、`alert`、および許可リスト内の `reason` のみを含みます。

- 回復シグナル: `healthy`
- 警告シグナル: `never_succeeded` / `run_failed` / `stale_heartbeat` / `retention_deadline` / `invalid_clock` / `incomplete_observation` / `monitoring_failed`

※ACK はあくまで「出力先での受理」を意味し、実際の通知配送や担当者による確認までは保証しません。受理済みの異常記録は出力先で保持し、後の正常回復で消去しない運用とします。元のエラーは内部で `unwrap` 可能な形で返しつつ、公開ログには出力しません。

---

## 合成検証と未確認事項

- ローカル検証: モック・ローカルパイプ・Firestore Emulator（[run-firestore-integration-tests.sh](../../../.github/scripts/run-firestore-integration-tests.sh)）を用いて、23 時間境界・再実行・キャンセル・出力先障害・破損状態の拒否・並行トランザクション・端数時刻の再送保護などを検証しています。
- 実環境で残っている確認事項: 実プロジェクト・ランタイム・所有者、実スケジューラーと独立監視の設定、通知出力先と配送到達、失敗時再試行、ストア・ログ・バックアップの一覧と保持設定（この文書やソースだけでは実環境への配備・有効化は行われません）。
