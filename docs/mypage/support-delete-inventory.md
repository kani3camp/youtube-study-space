# Support deletion B: source inventory

横断削除で調査するランタイム2種類と削除対象11種類を示す。実環境の完全な一覧や削除権限を示すものではない。対象環境の全リビジョン・手動起動・外部コピーを別途照合する。

| 固定 scope ID | 範囲 | 完了に必要な主な証跡 |
| --- | --- | --- |
| `runtime-my-page` | OAuth callback、account consume/session、mint、BFF cache/切断後の flight、metadata refresh/cleanup、support proof | 全 revision/instance/direct runner の admission と SDK/flight/commit/配達 drain |
| `runtime-legacy` | Bot、独立 force move、organizer、capacity、日次・手動 SFN/Fargate、BQ import、trend、admin、export/restore、通知 | 起動・再試行・再開経路の停止、実行中 task/job の終端、古い queue/snapshot の破棄・封鎖 |
| `seat-state` | `seats` / `member-seats` の target seat | `user-id` 帰属と revision 再照合、通常 Out/Block の副作用を生成しない専用消去 |
| `firestore-primary` | `users`（月次読取markerを含む）、work segment、`work-months`、`mypage-history-budgets`、activity、order、4 種 seat limit、legacy raw history | direct lookup と各 equality query の全件 pagination・再 scan・不存在。月次rebuildはoriginal SAC checkpointを各操作へ束縛 |
| `firestore-web` | WebAccount、process cache/flight、配達済み private display | account 不存在、全 process の stale publication 拒否、account 再作成との世代分離 |
| `firestore-oauth` | login/support OAuth、channel 未確定 callback | channel query、receipt relation、明示 transaction、未確定 callback の admission を合わせた閉路 |
| `firestore-support-relations` | 全 target receipt、challenge/request index、support OAuth | 他 receipt を含む ownership 照合、現在の完了依存を保持した関連 cleanup |
| `legacy-mappings` | 旧 channel owner / legacy uid account / linked Auth | 旧 UID の確認と逆・重複・移行関係、旧 writer/restore の確認 |
| `firebase-auth` | 現在・旧 mapped UID、mint/token/session | 明示 project/tenant/UID、mint と配達 drain、revoke/delete の結果と readback |
| `bigquery` | 3 本表、`tmp`、load/query/copy/export job と結果コピー | main/tmp 不存在、実行・再開可能な古い job/source が再投入しないこと |
| `backups-exports` | mixed GCS snapshot、generation/soft delete、managed backup/PITR、local JSON/export copies | snapshot integrity を維持した帰属・残存処置、recoverable copy と restore の確認 |
| `derived-vendor` | global trend/examples、OpenAI stored Responses、Discord/YouTube copies | channel index のない派生物の provenance、vendor ownership/削除・残存証跡。合成adapterは[削除手順](support-delete.md#派生データとvendor)参照 |
| `platform-logs` | local/CloudWatch/SFN/API/Hosting/Cloud Run/vendor logs、forwarded payload | 各 sink の実帰属と処置、遅延 forwarding drain、実設定の redaction |

Canon 05では、停止前に取得・受理した古いメッセージ、座席スナップショット、コールバック、トークン発行結果、import・restore・再試行が、削除済みデータを復活させないことを条件にする。再開後の正当な新規利用によるデータ作成は許容する。初期SACの対象はMyPageだけで、既存Botの通常書き込み、`!block`、YouTube Banの意味は変更しない。

`service-access-controls/{channel}`は横断消去の対象外とする。privacy解除後も、すべて無効なチェックポイント、両方の制限理由、世代、リビジョンを保持する。TTL・削除・一部だけの削除済みマーカー・世代リセットは追加しない。独立して有効なmoderationを保持し、復元・移行によるチェックポイントの巻き戻しも許容しない。これは新しい保持ポリシーの決定ではない。

プロセスログの秘匿処理と外部配送は別に調査する。トレンド・SNS・退室・RP・初期化のログを固定状態に変えても、OpenAIへの作業名送信、所有者・Discord・YouTubeへの通知、CloudWatch本文転送、旧リビジョンのコピーは残る場合がある。全出力先・キュー・購読・アーカイブの帰属、保持・保全、遅延配送を確認する。

完了受付と匿名の永続監査は、別の完了アダプター・ワークフローが扱う。この一覧自体は現在の受付を削除したり完了状態に変えたりしない。現在の証拠とインデックスの依存を保持し、全対象の事後条件、ランタイム再開、privacy解除の照合が成立してから、完了段階で処置する。

ソースだけでは次の公開条件を完了できない。全ランタイム・起動経路と停止・完了待ち・再開の管理者、チャンネル未確定コールバックとトークン発行・配送、複数受付と旧UID、BQのtmp/jobと混合・soft-deletedバックアップの復元経路、チャンネル索引のないトレンド・ベンダー・ログのコピー、実権限・インデックス/TTL・リビジョン・Hosting秘匿処理は未確認である。不明・上限超過・障害・ACK消失は`unknown`のまま残し、0・不存在・完了に変換しない。既存期間の保持や処置判断を推測せず、必要な実環境証跡を別途取得する。

## 起動と再生成経路の調査先

以下のソースを手がかりに、対象環境での稼働・手動実行・旧バイナリ・再試行・スケジューラーを列挙する。ソースがあることは、停止や実行中処理の完了を示す証拠ではない。

| writer | 調査先 |
| --- | --- |
| `mypage-oauth-start-callback` | [system/core/mypage/auth.go](../../system/core/mypage/auth.go) / [system/core/mypage/auth_store.go](../../system/core/mypage/auth_store.go) / [system/core/mypage/support_auth_store.go](../../system/core/mypage/support_auth_store.go) |
| `mypage-account-consume-session-mint` | [system/core/mypage/auth.go](../../system/core/mypage/auth.go) / [system/core/mypage/auth_store.go](../../system/core/mypage/auth_store.go) / [system/core/mypage/firebase_boundary.go](../../system/core/mypage/firebase_boundary.go) / [mypage/src/firebase.ts](../../mypage/src/firebase.ts) |
| `mypage-bff-cache-flight` | [system/core/mypage/bff.go](../../system/core/mypage/bff.go) / [system/core/mypage/http.go](../../system/core/mypage/http.go) / [system/cmd/mypage-server/main.go](../../system/cmd/mypage-server/main.go) |
| `mypage-metadata-refresh` | [system/core/mypage/metadata.go](../../system/core/mypage/metadata.go) / [system/core/mypage/metadata_store.go](../../system/core/mypage/metadata_store.go) |
| `mypage-metadata-cleanup` | [system/core/mypage/metadata_cleanup.go](../../system/core/mypage/metadata_cleanup.go) / [system/core/mypage/metadata_cleanup_store.go](../../system/core/mypage/metadata_cleanup_store.go) |
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

実行用の本人情報は、MyPageの鍵を使わないランタイム・署名者、Bot・旧管理ツールの認証情報ファイル、AWSランタイムのWIF・権限借用に分かれる。実主体・プロジェクトやテナント・最小権限・停止担当を環境ごとに照合し、1つの認証情報で全経路を止められるとは推測しない。
