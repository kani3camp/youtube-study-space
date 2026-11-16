# 現行ストアの Privacy 削除 CLI

この手順は、所有者が非公開の端末から個人データ削除を実施するためのものです。

> [!IMPORTANT]
> コードのマージやローカル試験は、実データ削除の承認や実環境受入の根拠にはなりません。受付から **7 暦日** の期限は、OAuth のやり直しや再試行で延長しないでください。

---

## 実装と確認の分担

`cmd/mypage-support-delete` は `--execute` を付けた場合のみ、所有者の既存 ADC / GCP IAM で固定プロジェクトへ接続します（実行用の新しい権限・SA・Operator OIDC は作りません）。

| 対象 | 処理内容と確認ポイント |
| --- | --- |
| **確認済み SupportRequest** | 初回 Acquire で目的・環境・チャンネル・fresh OAuth の証拠を照合し、proof claim・実行 checkpoint・`privacyDeletion` を同じトランザクションで確保します |
| **Firestore** | 現行の固定コレクションを直接操作します。対象クエリの先頭ページを繰り返し削除し、各ページで所有者・世代を照合して完了前に全ストアを再読み取りします |
| **Firebase Auth** | チャンネル ID と同じ UID のみを対象にします。無効化前に creation timestamp を保存して別アカウント生成（re-incarnation）の混入を防ぎ、削除後に不存在を再読します |
| **BigQuery** | 固定 `firestore_export` の 3 テーブルと `tmp` のみを対象にします。メタデータで存在・所有フィールドを確認し、パラメータ化 `DELETE`、安定ジョブ ID による ACK 消失照合、`COUNT` による対象 0 件確認を行います |
| **Writer / BFF キャッシュ** | 所有者が全実行系の停止・完了待ち（drain）、キャッシュ破棄、古いキュー・import・restore の排除を実確認し、非公開 Gate に記録します |
| **Legacy / Backup / Vendor / Log** | 自動削除済みとみなさず、所有者が実棚卸しと処置を行ってスコープ別の非公開記録へ参照を残します（未確認の `unknown` がある場合は停止） |

`supportdelete.Engine` は固定 29 checkpoint を順番に実行する短いループであり、任意のワークフロー登録機能や旧合成ストアは撤去済みです。

---

## 初回実行前の準備

1. [人的 Privacy 運用](support-operator.md) で受付を記録し、同じ目的・チャンネルの fresh YouTube OAuth を確認します（通常の Firebase セッションでは代用しません）。OAuth 記録の TTL 消去前に初回 checkpoint を確保してください（消失済みの場合は推測で作らず、新しい本人確認で整合を取るまで停止します）。
2. 対象環境・認証プロジェクト・チャンネル・`request` / `proof` / `execution` 参照を非公開に照合します（DEV は `test-youtube-study-space`、Production は `youtube-study-space`。認証情報ファイル・エミュレーター・矛盾するプロジェクト指定は CLI が拒否します）。
3. [削除対象一覧](support-delete-inventory.md) をもとに、稼働中リビジョン/インスタンス、配信 PC の Bot、AWS Batch / SFN、手動 runner、export / restore、ベンダー側のコピーを調査します（旧合成 checkpoint が実環境に存在しないことも配備前に確認します）。
4. パーミッション `0600` の通常ファイルにマニフェストを保存します（`schemaVersion: 1`、`operation: delete`、`target`、`requestRef`、`executionRef`、`proofRef`、同値の `confirmation` を設定。UID や参照を CLI 引数・公開ログ・PR に貼らないでください）。

```sh
# system/ で実行（既定の plan モードは ADC 探索・読み取り・変更を行いません）
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-delete --manifest /private/path/request.json

# 初回実行（privacyDeletion と checkpoint を確保し、Gate 未設定なら MANUAL_GATE_REQUIRED で停止）
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-delete --manifest /private/path/request.json \
  --execute --checkpoint /private/path/checkpoint.json
```

※Production 環境では環境変数・プロジェクトの両方を本番用に切り替え、`--allow-production` を追加します（これは操作意図の確認であり、対象データやバックアップの削除承認そのものではありません）。

---

## 所有者Gate

checkpoint の `selector` / `generation` / `cutoff` を完全一致で転記した `0600` JSON を作成し、`OwnerDeletionGates` スキーマで実測・確認結果を記録します（ファイル内の宣言だけを第三者検証や自動削除の証拠として公開しないでください）。

- **`expiresAt`**: 現在から 1 時間以内の期限（期限や世代が変わった場合は実状態を再確認）。
- **`writersStoppedAt`**: MyPage（mint / metadata / flight）、Bot、Batch、手動処理、import、Auth 作成を含む全 writer を停止し、開始済み処理の終了を確認した時刻。
- **`cachePurgedAt`**: 全 BFF インスタンス/リビジョンのキャッシュを破棄した時刻（保護開始後のインスタンス再起動と実行中処理の終了確認を用い、旧リビジョンを再稼働させない）。
- **`restoreExcludedAt`**: 古いキュー、スナップショット、copy / import / restore が対象を再生成しないことを確認した時刻（停止確認の 3 時刻は `cutoff` 以降にします）。
- **`manual`**: `legacy-mappings`、`backups-exports`、`derived-vendor`、`platform-logs` の各項目について、`status`（`completed` または不存在を調査した `absent`）、64 桁 16 進数の非公開 `evidenceRef`、`cutoff` 以降の `observedAt` を記録します（未確認なら `unknown` のまま停止）。
  - `legacy-mappings` の `absent` は、チャンネル UID 以外の旧 UID・テナント・共有アカウントが存在しないことまで含みます（存在した場合はこの CLI で推測削除せず、同じ依頼と期限のもとで別途帰属確認と処置を行います）。
- **`writersResumedAt`**: `resume` 段階に到達するまでは未設定/ゼロを必須とします（非ゼロなら副作用の前に拒否）。全 `inspect` 完了後に CLI が `resume` Gate で停止したことを checkpoint で確認し、古いジョブやキューを破棄したまま writer を再開・照合した時刻を checkpoint 更新時刻以降に記録します（早期再開が判明した場合は時刻だけ書き換えず、全 writer の停止・drain と再生成データ調査をやり直します）。

```sh
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-delete --manifest /private/path/request.json \
  --execute --checkpoint /private/path/checkpoint.json --gates /private/path/gates.json
```

### 部分失敗と checkpoint ファイルの取り扱い

- 部分失敗時は `privacyDeletion` を有効のまま残し、同じマニフェストと非公開 checkpoint ファイルで再実行します（進捗の正本は Firestore 上の状態）。
- checkpoint ファイルはランダムな owner 識別子を保持し、実行中は排他ロックされます。別ファイルで同じ実行へ参加すると owner 競合で停止し、`cursor` の編集によるスキップもできません。
- 実行中のコピー・置換・別端末への持ち出しは禁止です。ファイルが消失・破損した場合は owner を推測せず、全旧 CLI や依存ジョブの停止を確認して所有者回復を解決するまで保護を維持します。
- 現在の OAuth 記録と proof claim は最終トランザクションまで保持し、初回確保後は TTL 消失によって取得済み対象を別チャンネルへ切り替えません。

---

## 完了・復旧

- **完了処理**: 全ストアの `inspect` と writer 再開確認が揃った後にだけ、現在の申請・実行・SAC 世代をトランザクションで再照合し、Auth generation 記録や OAuth・関連参照を消去して最小限の完了記録へ置き換え、`privacyDeletion` を解除します（`moderation` と SAC 世代は独立して保持します）。完了 ACK 消失時は完了 binding を読み、再削除へ戻らないようにします。
- **本番適用と復旧方針**: 本番実施前に DEV 専用データで失敗・再開・対象隔離・ロールバックを実受入します。CLI の中断や不明な結果が出たときに保護を手動解除しないでください。削除済みデータの復元は前提とせず、サービス停止・保護維持・同一 checkpoint からの前進復旧を行います。
- **個別削除不可な領域の扱い**: バックアップ・PITR・soft-delete・エクスポート・ベンダー領域で個別削除ができない場合、本番名前空間からの不在と完全削除（hard deletion）を区別し、7 暦日要件と整合しない復元可能なコピーが残るなら `completed` にはしません。

※検証責任の分担：トランザクション原子性・対象隔離・世代・CAS は Emulator、Auth 再作成と resume Gate は current-store integration、BQ ジョブ・パラメータ・再読はローカル SDK HTTP 試験、ACK 消失・再開位置・完了後の新規利用保護は checkpoint 単体テストで担保しています（実環境 Gate の完了は別途確認が必要です）。
