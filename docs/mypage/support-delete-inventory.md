# Support deletion B: source inventory

[support-delete-inventory.json](support-delete-inventory.json) は、B の実行 workflow が停止・drain・横断削除をレビューするための静的ソース台帳。2026-10-07 に repository の実装と privacy runbook を読み、固定の runtime scope 2 件、削除 scope 11 件、writer 群 17 件を対応づけた。外部 export/backup/restore 群は実装・稼働が未確認の候補として区別する。

各 scope / writer はソース位置と symbol、作成・再作成経路、帰属方法と限界、必要な pause/drain/verify 証跡、既存実装と未実装・実環境未確認の境界を持つ。`sourceKnown` はソースの事実、`liveUnknown` は対象環境で確認が必要な事項。`requiredEvidence` は要求であり、取得済みの証跡ではない。実データ・credential は参照せず、SDK・ネットワーク・停止・revoke・削除・deploy 操作を実行していない。

| 固定 scope ID | 範囲 | 完了に必要な主な証跡 |
| --- | --- | --- |
| `runtime-my-page` | OAuth callback、account consume/session、mint、BFF cache/切断後の flight、metadata refresh/cleanup、support proof | 全 revision/instance/direct runner の admission と SDK/flight/commit/配達 drain |
| `runtime-legacy` | Bot、独立 force move、organizer、capacity、日次・手動 SFN/Fargate、BQ import、trend、admin、export/restore、通知 | 起動・再試行・再開経路の停止、実行中 task/job の終端、古い queue/snapshot の破棄・封鎖 |
| `seat-state` | `seats` / `member-seats` の target seat | `user-id` 帰属と revision 再照合、通常 Out/Block の副作用を生成しない専用消去 |
| `firestore-primary` | `users`、work segment、activity、order、4 種 seat limit、legacy raw history | direct lookup と各 equality query の全件 pagination・再 scan・不存在 |
| `firestore-web` | WebAccount、process cache/flight、配達済み private display | account 不存在、全 process の stale publication 拒否、account 再作成との世代分離 |
| `firestore-oauth` | login/support OAuth、channel 未確定 callback | channel query、receipt relation、明示 transaction、未確定 callback の admission を合わせた閉路 |
| `firestore-support-relations` | 全 target receipt、challenge/request index、support OAuth | 他 receipt を含む ownership 照合、現在の完了依存を保持した関連 cleanup |
| `legacy-mappings` | 旧 channel owner / legacy uid account / linked Auth | 旧 UID の確認と逆・重複・移行関係、旧 writer/restore の確認 |
| `firebase-auth` | 現在・旧 mapped UID、mint/token/session | 明示 project/tenant/UID、mint と配達 drain、revoke/delete の結果と readback |
| `bigquery` | 3 本表、`tmp`、load/query/copy/export job と結果コピー | main/tmp 不存在、実行・再開可能な古い job/source が再投入しないこと |
| `backups-exports` | mixed GCS snapshot、generation/soft delete、managed backup/PITR、local JSON/export copies | snapshot integrity を維持した帰属・残存処置、recoverable copy と restore の確認 |
| `derived-vendor` | global trend/examples、OpenAI stored Responses、Discord/YouTube copies | channel index のない派生物の provenance、vendor ownership/削除・残存証跡。合成adapterは[別文書](support-delete-derived-adapter.md)参照 |
| `platform-logs` | local/CloudWatch/SFN/API/Hosting/Cloud Run/vendor logs、forwarded payload | 各 sink の実帰属と処置、遅延 forwarding drain、実設定の redaction |

Canon 05 の境界は、停止前に取得・受付した古い message、seat snapshot、callback、mint/result、import/restore/retry が削除済みデータを復活させないこと。再開後の正当な新しい利用による新規データ作成は許容する。初期 SAC は MyPage のみで、既存 Bot の通常 writer、`!block` と YouTube Ban の意味を変更しない。

`service-access-controls/{channel}` は横断消去の対象から外し、privacy clear 後も **全 inactive checkpoint と両 reason state、generation、revision を保持する**。TTL、削除、subset tombstone、generation reset を追加しない。独立した active moderation を保持し、restore/migration による checkpoint 巻き戻しも許容しない。これは新しい retention policy の決定ではない。

現行ソースの trend・SNS 通知・一括退室経路は、作業名、モデル応答、SNS 件名/本文、退室通知文をログに転記せず、件数・配送状態・固定の失敗種別を記録する。近接する owner 通知と CloudWatch 転送の配送失敗ログも、provider のエラー本文を記録しない。通知本文の Discord 配送、CloudWatch 内容の転送、OpenAI への作業名送信は従来どおりである。旧 revision の実行、既存ログ、転送済み通知、他の legacy ログ経路の帰属・残存処置は未確認であり、この source 修正を `platform-logs` の削除証跡として扱わない。

この追加 source 修正の範囲は次のとおり。プロセスログと配送・返却データは別経路として扱う。

| 経路 | プロセスログで固定した内容 | 意図して維持する経路 |
| --- | --- | --- |
| `batch.go` の RP 更新・重複判定、export folder 解決 | channel ID と GCS folder path を除き、開始・skip reason・解決イベントを記録 | RP 更新の transaction と error identity、失敗時 owner 通知に含む既存の user ID/error |
| `utils.go` の座席制限・活動取得・退室検算と完了 | channel ID、作業名、個人の時間/RP 値を除き、固定 status と活動件数だけを記録 | seat/activity/work segment の書込、返却エラー、検算失敗時の owner 通知内容 |
| `youtube_organize_database`、`cmd/batch`、`youtube-bot` の該当入口 | OrganizeDB の生エラーと、Firestore option／WorkspaceApp 初期化エラーを固定 `error_class` に置換 | Lambda の従来の OK/timeout/継続分岐、batch の exit status、owner への元エラー通知 |
| `error_log_notify_discord` の parse・Firestore option・WorkspaceApp 初期化 | 生の依存エラーを除き固定 `error_class` を記録。返却エラーの表示も固定し、`errors.Is` から元の原因を照合可能にする | CloudWatch log body/logGroup/request ID の Discord 転送と、配送失敗時の non-nil error |

対象は上記の直接ログであり、owner/Discord や YouTube の配送内容を redaction した証拠ではない。`start_daily_batch` など他の起動経路・legacy logger、旧 binary、CloudWatch/SNS の queue・subscription・archive、実 retention/hold と対象帰属は引き続き `platform-logs` の実環境 inventory と処置証跡を要する。

完了 receipt と匿名 durable audit は別の completion adapter / workflow が担当する。この台帳は現在 receipt を削除したり completed に変更したりしない。現在の proof/index 依存を保持し、全 scope postcondition と runtime 再開・privacy clear の照合が成立してから、その completion 段階で処置する。

ソースだけでは次の release gate を閉じられない。全 runtime/launch 経路と stop/drain/resume ownership、channel 未確定 callback と token mint/配達、複数 receipt と旧 UID、BQ tmp/job と mixed/soft-deleted backup の復元経路、channel index のない trend/vendor/log コピー、実権限・index/TTL・revision・hosting redaction が未確認。未知・上限超過・障害・ack loss は `unknown` として残し、0、不存在、completed に変換しない。既存期間の保持・処置判断を推測せず、必要な実環境証跡を別に取得する。

ローカルで strict JSON parse、固定 scope / writer ID の重複、scope と writer の双方向対応、全 source path / anchor / line を確認した。これは実環境 inventory、停止能力、削除権限、実 Auth/backup/vendor 不存在の検証とは別である。
