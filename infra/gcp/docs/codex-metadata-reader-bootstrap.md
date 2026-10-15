# 専用の設定読取SAへ不足する読取権限を追加する準備物

[`bootstrap_codex_metadata_readers.py`](../scripts/bootstrap_codex_metadata_readers.py) は、既存の専用Codex dev metadata SAに不足する設定読取権限だけを追加する差分を検査します。
通常GCPログイン経由の作業は本人が継続的に許可しています。具体的な対象・差分・source・実行条件は、別のprivate packetと独立reviewで限定します。
このsourceの追加やoffline PASSだけで、実行済み・専用SAのread成功・fresh full7・State/root/import準備完了とは扱いません。

## 固定した変更範囲

| 対象 | 変更 |
| --- | --- |
| 既存dev Runtime Pool | 既存 `terraformDevRuntimePoolReadV1` bindingのCI plan/apply 2 memberを保持し、専用metadata SA 1 memberだけ追加 |
| 既存dev Runtime SA | 既存 `terraformDevRuntimeSaPolicyReadV1` bindingの同じCI 2 memberを保持し、同じ専用metadata SA 1 memberだけ追加 |
| dev custom role定義 | `terraformDevHistoryMetadataReadV1` を新規作成。含めるpermissionは `bigquery.tables.get` だけ |
| 保持中のdev history Table 1件 | 新roleを同じ専用metadata SA 1 memberだけに付与 |

既存2 roleの全定義は[先行の設定source](../scripts/bootstrap_runtime_readers.py)と完全一致を要求します。
Pool roleはPool GET・attestation rule GET・Provider GET、SA roleはRuntime SAのgetIamPolicyだけです。
Pool配下のProvider全体が粒度であり、単一Providerへ絞れたとは主張しません。
role定義のnamespaceはdev projectですが、Project IAM policyへのgrantはありません。
変更は新role 1件とpolicy 3件、member出現数3件、最大4 write**試行**です。
同名roleが同値でも再利用・patchせず停止します。既存roleも更新しません。

Project/Dataset IAM、行データ、query/job、Secret、API有効化、prod、新SA、trust、Terraform、State/lock、CIへの変更権限委譲は含みません。
元AWSキーを使う変更経路もありません。変更の復元・role削除・自動rollback・再送を行う経路はありません。

## private packetと既定動作

既定はofflineです。認証・HTTP・writeを行わず、manifestのhash、schema、固定対象、全role定義と限定差分を検査してprivate結果を保存します。
引数の正本はsourceのparser、manifestの正本は `MANIFEST_KEYS` / `validate_manifest` です。
専用metadata SAのemail/resourceはprivate入力とし、公開sourceに実値や推測した名前を埋め込みません。
既存の正本から担当者がpacketを一括生成し、利用者に識別子の手入力を求めません。

manifestは別kindの `private-owner-codex-metadata-reader-extension-intent` / schema 1です。
旧captureと専用SA正本のhashは住所の出典だけに使用します。古いpolicy、状態、成功flag、時刻をfreshな証拠へ転用しません。
script自身・変更しない同梱source・同revisionのworkflow・review済みintegration HEADをpinします。
元helperは隣接ファイルとして必要で、公開固定SHA256に合ったbytesだけをcompileします。未検証のimportやhash確認後の再読込はしません。
入力はcheckout外のcanonical absoluteなowned regular file、4MiB以下、hardlink/symlinkなし、重複JSON key・未知fieldなしを要求します。
0644でuploadされた入力はfdで0600へ制限し、親はownedかつgroup/other-write不可を要求します。

明示したlive-plan/execute modeは、source pinと通常user session経路の確認、既知IAM writerの変更静止判断がなければ認証前に停止します。
manifest有効期間は最大8時間、実際の認証待ち・read・write・保存は呼出開始から最大1時間です。
gcloudの現在の通常user sessionだけをaccount指定で使い、新login、SA account、impersonation、ADC/credential/token/login-config overrideを認めません。
短期tokenとCLI stdout/stderrはメモリだけで扱います。account選択やpermission probeを本人の独立identity証明へ読み替えません。

## 全検査を先に完了する

1. review済みintegration HEAD・workflow hash、明示した関連gate閉鎖とread時刻空欄、既知 `ci.yml` のactive dispatchなしを確認します。公開APIの403や不明応答は停止します。State専用emitterに一律falseは要求せず、全writer静止が証明されたとも扱いません。
2. fixed dev project GETでID/number/ACTIVEを照合し、専用metadata SA GETでexact resource・projectId・email・uniqueId・disabled falseを照合します。SA APIに架空のType fieldは要求しません。CI 2 SAとRuntime SAは対象から除外します。
3. actual tokenについてrole create/get、exact Pool/Runtime SA/Tableの必要なpolicy get/set、およびTable metadata get permissionをprobeします。probeは観測であり、非存在resourceや将来requestの認可保証ではありません。
4. 既存2 roleをexact GETし全定義を照合、新history roleはexact GETの404だけを許します。
5. Pool/Runtime SA/Tableのfull IAM policyをrequested version3で取得します。全3件に非空の**IAM Policy etag**が必須です。空policyやetag欠落はwrite前に停止し、先行helperの空Pool例外を再利用しません。Table metadataのetagを使う経路もありません。
6. Pool/SAの対象role bindingは無条件の各1件で、既存memberがCI 2個だけであることを確認します。追加SAが既存、対象binding複数・条件付き・別memberあり、history roleのbinding既存も停止します。無関係なbinding、condition、auditConfigはそのまま保持します。

正本に過去のuniqueIdをpinしていない場合、同名再作成を過去と独立比較したとは扱いません。
呼出中はfresh SA応答を保持し、各write前の再GET完全一致で同じuniqueIdを要求します。

## 4試行と競合時の停止

全preflight後、新history role作成→Pool→Runtime SA→Tableの順に各1回だけ送信します。
各write前にGitHub HEAD/gate/activity、専用SAの同一性を再確認します。
各policy SET直前に対象roleの定義とfull policyを再GETし、preflightとの完全一致と同じIAM etagを要求します。
古いsnapshotへの新しい変更の自動mergeは行いません。
SETは元etagを含むfull policyと `updateMask=bindings,etag` を使い、SET応答と事後GETの両方を予定した差分だけと照合します。
member/bindingの順序と、条件なしpolicyのversion 1/3応答差だけを正規化し、条件付きversion3とauditConfig・既存内容を保持します。

送信前にwrite試行をjournalへ記録・fsyncし、fsync後にも期限を確認します。
HTTP409、timeout、未知結果、不正なHTTP200本文、事後差分の不一致は即停止です。最大4回は成功数ではなく予約した試行数です。
role作成だけが確認済みの場合も、残りの設定を完了扱いにしません。再実行・復旧はpartial journalの独立reviewを要する別の判断です。
etagは同一policy上の競合を防ぎますが、別role定義や未知writer全体のロックではありません。

## private結果と証拠の限界

結果directoryは0700、journal/snapshotは0600・exclusiveで作成し、公開出力は固定status/mode/試行数、falseのmigration-ready、private結果pathだけです。
保存対象は `journal.jsonl`、`offline-intent.json`、`preflight.json`、`dry-plan.json`、`write-1.json`〜`write-4.json`、`after-pool-policy.json`、`after-sa-policy.json`、`after-history-policy.json` の必要なsubsetです。
token、CLI出力、HTTP error body、未検証の成功本文は保存・公開しません。private policyをrepositoryや公開artifactへ移しません。
journalの開始・完了・各eventにはUTCを付けますが、時計は **local-clock-self-report** であり独立実行時刻の証明ではありません。
setup完了と、専用identityの実際のread成功・fresh full7・migration/import readyは区別し、後者は常にfalseです。
fresh full7の収集とprivate受渡しは、このhelperとは別のreview対象です。

## 検証と公式根拠

実認証なしのダミー検証: `python3 infra/gcp/tests/test_bootstrap_codex_metadata_readers.py`。
既存helperの検証も維持し、scope、etag、role衝突、CI保持、競合、期限、partial/UNKNOWN、source pin、default offlineを確認します。

- [BigQuery Tableへのcustom role付与とpermissionの最小resource](https://docs.cloud.google.com/bigquery/docs/control-access-to-resources-iam)
- [Table getIamPolicy](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/getIamPolicy)、[GetPolicyOptions](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/GetPolicyOptions)
- [Table setIamPolicy](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/setIamPolicy)、[Policy etag](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/Policy)
- [Table testIamPermissionsの証拠限界](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/testIamPermissions)
- [custom role permission support](https://docs.cloud.google.com/iam/docs/custom-roles-permissions-support)
