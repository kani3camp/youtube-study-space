# Privacy 横断削除の対象一覧（ソース棚卸し）

横断削除で調査対象とする **2 種類のランタイム** と **11 種類の削除スコープ** の一覧です。

> [!NOTE]
> これはソースコード上の調査マップであり、実環境の完全な棚卸しや削除権限の有無を示すものではありません。対象環境の全リビジョン・手動起動・外部コピーは別途照合してください。

## 調査スコープ一覧

| 固定 scope ID | 調査範囲 | 完了に必要な主な確認項目 |
| --- | --- | --- |
| `runtime-my-page` | OAuth callback、アカウント消費・セッション、トークン発行、BFF キャッシュ・切断後の処理、メタデータ更新・削除、サポート本人確認 | 全リビジョン・インスタンス・直接実行の新規受付停止と、SDK / flight / commit / 配達中の処理完了（drain） |
| `runtime-legacy` | Bot、独立 force move、organizer、座席数制御、日次・手動 SFN / Fargate、BQ import、trend、admin、export / restore、通知 | 起動・再試行・再開経路の停止、実行中タスク/ジョブの終了、古いキュー・スナップショットの破棄・封鎖 |
| `seat-state` | `seats` / `member-seats` の対象座席 | `user-id` 帰属とリビジョン再照合、通常の退室/ブロックの副作用を起こさない専用消去 |
| `firestore-primary` | `users`（月次読取マーカー含む）、work segment、`work-months`、`mypage-history-budgets`、activity、order、4 種 seat limit、旧生履歴 | 直接参照および各等値クエリの全件ページング・再走査・不存在確認（月次再構築は元の SAC チェックポイントを各操作へ束縛） |
| `firestore-web` | WebAccount、プロセスのキャッシュ/flight、配信済みの画面データ | アカウント不存在、全プロセスでの古い出力の抑止、アカウント再作成との世代分離 |
| `firestore-oauth` | ログイン/サポート用 OAuth、チャンネル未確定 callback | チャンネルクエリ、受付関連、明示トランザクション、未確定 callback の受付制御を合わせた閉鎖確認 |
| `firestore-support-relations` | 対象者の全受付レコード、チャレンジ/リクエストインデックス、サポート OAuth | 他受付を含む所有権照合、完了依存関係を保ったうえでの関連整理 |
| `legacy-mappings` | 旧チャンネル所有者 / 旧 UID アカウント / 連携済み Auth | 旧 UID の有無と逆引き・重複・移行関係、旧 writer / restore の確認 |
| `firebase-auth` | 現行チャンネル UID、トークン発行・セッション（旧 UID / テナントは legacy Gate） | プロジェクト・テナント・UID の明示、発行と配信の drain、無効化・削除結果の再読み取り |
| `bigquery` | 3 テーブル、`tmp`、load / query / copy / export ジョブと結果コピー | 本表・`tmp` からの不存在、実行中・再開可能な古いジョブ/ソースから再投入されないこと |
| `backups-exports` | 混合 GCS スナップショット、generation / soft delete、マネージドバックアップ / PITR、ローカル JSON / export コピー | スナップショット整合性を保った帰属確認・残存処置、復元可能コピーとリストア経路の確認 |
| `derived-vendor` | 全体トレンド・例文、OpenAI 保存 Responses、Discord / YouTube 上のコピー | チャンネル索引のない派生データの由来、外部サービスの所有・削除・残存確認（個別処置は [所有者 Gate](support-delete.md#所有者gate) を参照） |
| `platform-logs` | ローカル / CloudWatch / SFN / API / Hosting / Cloud Run / 外部サービスログ、転送ペイロード | 各出力先の実帰属と処置、遅延転送の drain、実設定のマスキング確認 |

---

## 設計上の境界・未確認事項

- データ復活の防止（Canon 05）: 停止前に取得・受理した古いメッセージ、座席スナップショット、コールバック、トークン発行結果、import / restore / 再試行によって削除済みデータが復活しないことを条件とします（再開後の正当な新規利用によるデータ作成は許可）。初期 SAC の対象は MyPage のみとし、既存 Bot の通常書き込み・`!block`・YouTube Ban の意味は変えません。
- `service-access-controls/{channel}` は消去対象外: プライバシー削除完了後もすべて無効状態のチェックポイント・両方の制限理由・世代・リビジョンを保持します（TTL・削除・世代リセットは追加せず、有効な `moderation` やチェックポイントの巻き戻しも行いません。これは保持ポリシーの新規決定ではありません）。
- ログと外部配送の調査: トレンド・SNS・退室・RP・初期化ログを固定形式に変えても、OpenAI への作業名送信、Discord / YouTube 通知、CloudWatch 本文転送、旧リビジョンのコピーが残る場合があります。全出力先・キュー・購読・アーカイブの帰属と保持を確認してください。
- 完了処理の順序: この棚卸し一覧自体は受付を削除・完了させません。全対象の事後条件・ランタイム再開・privacy 解除の照合が成立してから完了段階で処理します。
- ソースだけでは完了できない項目: 全ランタイムの停止担当、未確定コールバックと配信、複数受付・旧 UID、BQ の `tmp` / 混合・soft-delete バックアップの復元経路、チャンネル索引のない派生・ログコピー、実権限・TTL・Hosting 秘匿設定は実環境で確認が必要です。不明・上限超過・障害・ACK 消失は `unknown` のまま残し、`0` や完了に置き換えないでください。

---

## 起動と再生成経路の調査先

対象環境での稼働プロセス・手動実行・旧バイナリ・再試行・スケジューラーを列挙するためのソース上の参照先です（ソースの存在は停止完了を示すものではありません）。

| Writer | 調査先のソースコード |
| --- | --- |
| `mypage-oauth-start-callback` | [system/core/mypage/auth.go](../../system/core/mypage/auth.go) / [system/core/mypage/auth_store.go](../../system/core/mypage/auth_store.go) / [system/core/mypage/support_auth_store.go](../../system/core/mypage/support_auth_store.go) |
| `mypage-account-consume-session-mint` | [system/core/mypage/auth.go](../../system/core/mypage/auth.go) / [system/core/mypage/auth_store.go](../../system/core/mypage/auth_store.go) / [system/core/mypage/firebase_boundary.go](../../system/core/mypage/firebase_boundary.go) / [mypage/src/firebase.ts](../../mypage/src/firebase.ts) |
| `mypage-bff-cache-flight` | [system/core/mypage/bff.go](../../system/core/mypage/bff.go) / [system/core/mypage/http.go](../../system/core/mypage/http.go) / [system/cmd/mypage-server/main.go](../../system/cmd/mypage-server/main.go) |
| `mypage-metadata-refresh` | [system/core/mypage/metadata.go](../../system/core/mypage/metadata.go) / [system/core/mypage/metadata_store.go](../../system/core/mypage/metadata_store.go) |
| `mypage-metadata-cleanup` | [system/core/mypage/metadata_cleanup.go](../../system/core/mypage/metadata_cleanup.go) / [system/core/mypage/metadata_cleanup_store.go](../../system/core/mypage/metadata_cleanup_store.go) / [system/cmd/mypage-metadata-cleanup/main.go](../../system/cmd/mypage-metadata-cleanup/main.go) |
| `mypage-support-proof-indices` | [system/core/mypage/support_store.go](../../system/core/mypage/support_store.go) / [system/core/mypage/support_auth_store.go](../../system/core/mypage/support_auth_store.go) / [system/core/mypage/auth.go](../../system/core/mypage/auth.go) |
| `legacy-bot-chat-commands` | [system/cmd/youtube-bot/main.go](../../system/cmd/youtube-bot/main.go) / [system/core/workspaceapp/workspace_app.go](../../system/core/workspaceapp/workspace_app.go) / [system/core/workspaceapp/utils.go](../../system/core/workspaceapp/utils.go) / [system/core/workspaceapp/command_seat.go](../../system/core/workspaceapp/command_seat.go) / [system/core/workspaceapp/command_user.go](../../system/core/workspaceapp/command_user.go) |
| `legacy-force-move-long-sitting` | [system/cmd/youtube-bot/main.go](../../system/cmd/youtube-bot/main.go) / [system/core/workspaceapp/workspace_app.go](../../system/core/workspaceapp/workspace_app.go) / [system/core/workspaceapp/batch.go](../../system/core/workspaceapp/batch.go) / [system/core/workspaceapp/utils.go](../../system/core/workspaceapp/utils.go) |
| `legacy-seat-organizer` | [system/cmd/lambda/youtube_organize_database/main.go](../../system/cmd/lambda/youtube_organize_database/main.go) / [system/core/workspaceapp/batch.go](../../system/core/workspaceapp/batch.go) / [aws-cdk/lib/aws-cdk-stack.ts](../../aws-cdk/lib/aws-cdk-stack.ts) |
| `legacy-capacity-control` | [youtube-monitor/src/components/MainContent.tsx](../../youtube-monitor/src/components/MainContent.tsx) / [system/cmd/lambda/set_desired_max_seats/main.go](../../system/cmd/lambda/set_desired_max_seats/main.go) / [system/core/workspaceapp/max_seats_adjustment.go](../../system/core/workspaceapp/max_seats_adjustment.go) |
| `legacy-daily-manual-batch` | [system/cmd/batch/main.go](../../system/cmd/batch/main.go) / [system/core/workspaceapp/batch.go](../../system/core/workspaceapp/batch.go) / [system/cmd/lambda/start_daily_batch/main.go](../../system/cmd/lambda/start_daily_batch/main.go) / [aws-cdk/lib/aws-cdk-stack.ts](../../aws-cdk/lib/aws-cdk-stack.ts) |
| `legacy-bigquery-import` | [system/core/workspaceapp/batch.go](../../system/core/workspaceapp/batch.go) / [system/core/mystorage/storage_controller.go](../../system/core/mystorage/storage_controller.go) / [system/core/mybigquery/bigquery_controller.go](../../system/core/mybigquery/bigquery_controller.go) |
| `legacy-work-name-trend` | [system/core/workspaceapp/work_name_trend.go](../../system/core/workspaceapp/work_name_trend.go) / [system/cmd/lambda/update_work_name_trend/main.go](../../system/cmd/lambda/update_work_name_trend/main.go) / [aws-cdk/lib/aws-cdk-stack.ts](../../aws-cdk/lib/aws-cdk-stack.ts) |
| `legacy-admin-and-local-export` | [system/internal/adminops/operations.go](../../system/internal/adminops/operations.go) / [system/core/workspaceapp/utils.go](../../system/core/workspaceapp/utils.go) / [system/core/workspaceapp/batch.go](../../system/core/workspaceapp/batch.go) |
| `legacy-raw-chat-helper-and-old-binary` | [system/core/workspaceapp/utils.go](../../system/core/workspaceapp/utils.go) / [system/core/repository/firestore_controller.go](../../system/core/repository/firestore_controller.go) / [system/core/mybigquery/bigquery_controller.go](../../system/core/mybigquery/bigquery_controller.go) |
| `legacy-notification-log-forwarding` | [system/core/workspaceapp/utils.go](../../system/core/workspaceapp/utils.go) / [system/core/workspaceapp/command_moderation.go](../../system/core/workspaceapp/command_moderation.go) / [system/core/youtubebot/live_chat.go](../../system/core/youtubebot/live_chat.go) / [system/core/moderatorbot/discord.go](../../system/core/moderatorbot/discord.go) / [system/cmd/lambda/error_log_notify_discord/main.go](../../system/cmd/lambda/error_log_notify_discord/main.go) / [system/cmd/lambda/sns_notify_discord/main.go](../../system/cmd/lambda/sns_notify_discord/main.go) |
| `external-export-backup-restore` | [system/core/repository/models.go](../../system/core/repository/models.go) / [system/core/mystorage/storage_controller.go](../../system/core/mystorage/storage_controller.go) |

※実行用の認証主体は MyPage の鍵なしランタイム/署名者、Bot・旧ツールの認証ファイル、AWS ランタイムの WIF/IAM ロールに分かれています。1 つの認証情報で全経路を止められるとはみなさず、環境ごとに実主体・最小権限・停止担当を照合してください。
