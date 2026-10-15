# 保持history Tableの読取設定だけを完了する準備物

[`complete_codex_history_reader.py`](../scripts/complete_codex_history_reader.py) は、既存の専用Codex dev metadata SAに対する、保持history Table 1件の `bigquery.tables.get` 付与だけを扱います。
先行結果は新role作成・Pool/Runtime SA更新の3件が確認済み、Table更新の第4試行がHTTP400でSTOPでした。
元の4変更commandは再実行せず、確認済み変更のrollbackもしません。今回のsource追加は、新たな実行や成功を表すものではありません。

## 固定する差分

既存 `terraformDevHistoryMetadataReadV1`（permissionは `bigquery.tables.get` 1個）を、固定dev history Tableに、同じ専用metadata SA 1 memberだけ付与します。
role作成・更新やPool/Runtime SA更新はありません。TableのSET試行は最大1回です。
Project/Dataset IAM、行データ、jobs、Secret、trust、新SA、API有効化、State/lock、Terraform、prodは対象外です。
通常GCPログイン経由の許可と、具体的な新packetのsource・対象・差分・独立reviewは区別します。

Table固有の[公式手順](https://docs.cloud.google.com/bigquery/docs/control-access-to-resources-iam)はnative version 1の保持を指示します。
先行sourceはhistoryも一律version 3へ変えていました。[一般Policy仕様](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/Policy)は無条件ならversion 3も許容するため、これだけで先行400の原因を確定しません。
先行helperのhistory計画も、native version 1を保持し、version 3や条件付きpolicyを変更前に拒否するよう修正します。Pool/Runtime SAの計画と未変更base helperは維持します。

## offlineとprivate manifest

既定はofflineで、認証・HTTP・writeは0です。mandatory manifest path/hashをprivate path規則で検査し、固定intentをprivate保存します。
manifestは別kind `private-owner-history-reader-completion-intent` / schema 1です。
正確なkeyはsourceの `MANIFEST_KEYS` / `validate_manifest` を正本とし、主に次を含みます。

- 独立data review済みの旧結果ZIP hash、canonicalな旧intent全文/hash。旧source hashは56fad版へ固定し、旧時刻をfreshへ延長しません。
- review済み `expected_snapshot`：project、専用SA full response/uniqueId、既存3 role全文、Pool/Runtime SA更新後full policy、元のhistory full IAM policy。
- 新source・guard修正後reader・未変更base・同revision workflowのhash、review済みintegration HEAD。
- 通常user session scope、owner routeと既知writer変更静止の判断。manifest最長8時間、呼出開始から実処理最大1時間。

旧ZIP自体はこのhelperへ再入力しません。旧ZIPとexpected snapshotのjoinは担当者のprivate packet生成と独立data reviewで行い、helper内では旧intentのcanonical hashと固定schemaを確認します。
古いsnapshotを現在の証拠として発行せず、新しいGET応答との照合にだけ使います。
同梱2 moduleは公開固定hashへ合うbytesだけをcompileします。未検証import、元キー/SA key探索、新login、認証設定変更はありません。
入力はcheckout外のowned regular600、canonical absolute・symlink/hardlinkなし・4MiB以下・未知field/重複JSON keyなしを要求します。upload0644はfdで600へ制限します。

明示したlive-plan/executeにはsource pinと新manifestのACK2件が必要です。offlineのPASSや過去のACKは実行承認の代わりになりません。
実行案内と実値packetはsource・差分・独立review完了後に担当者がまとめます。ここでは実行commandを提示しません。

## 全検査後に0回または1回だけ変更する

1. 固定HEAD/workflow hash、明示した関連gate閉鎖・read時刻空欄、既知CI active dispatchなしを確認します。API拒否や不明なら停止し、全writer不在を証明したとは扱いません。
2. fixed dev project GETと専用SA GETでID/number/ACTIVE、email/resource/disabled false/同じuniqueIdを照合します。
3. 既存3 roleの全定義とetag、Pool/Runtime SAのfull policyをreview済みsnapshotへ完全一致させます。対象bindingは無条件各1件、CI 2 member＋同じmetadata SA 1 memberです。無関係なbinding/condition/auditConfigも保持します。
4. Table full IAM policyはrequested version3で取得して条件付きpolicyの見落としを避けますが、応答のnative versionは1・条件なし・非空IAM etagを必須にします。Table metadata etagは取得も代用もしません。
5. owner tokenのTable policy get/setだけをprobeします。role create authorityは要求しません。probeだけで存在・認可を証明したとは扱いません。

Table policyが元snapshotと完全一致なら、元IAM etagを保持して1 bindingだけを追加します。
既に予定どおりの場合はSETせず、新しいowner readの整合確認だけを記録します。過去の400試行を成功へ変更しません。
その他の漂流や競合は停止します。

送信直前にもHEAD/gate/activity、全role・全policy・同じSAを再GETして完全一致を要求します。
write-attemptをdurable journalへ記録・fsyncした後にもう一度期限を検査し、標準CLIと同じ `{"policy": fullPolicy}` bodyで、固定TableへPOSTを1回だけ送ります。
`updateMask`の省略はRESTの既定 `bindings,etag` を使う選択で、CASを省略する意味ではありません。
応答と事後Table GETを予定差分だけと照合し、最後に全role/policy/SAを再確認します。
409・timeout・HTTP400・未知結果・不正な成功本文・事後差分不一致は停止し、再送・自動rollbackをしません。

## 診断とprivate結果

失敗SETのerror bodyは最大64KiBまでメモリ内だけで厳密JSON解析します。
journalへ残すのはnumeric HTTP/error codeと、source固定のstatus/reason enumだけです。
未知のstatus/reason文字列、message、error body、identity、tokenは保存・表示しません。enumから原因を断定しません。

結果directoryは `yss-history-reader-completion-*`（700）、固定結果fileは各exclusive600です。
保存するsubsetは `journal.jsonl`、`offline-intent.json`、`preflight.json`、`dry-plan.json`、`write-1.json`、`after-history-policy.json`、`after-final.json` です。
stdoutは固定status/mode/試行数0または1/migration-ready=falseと、存在すればprivate RESULT_DIRECTORYだけです。
UTCはlocal-clock-self-reportであり独立実行時刻ではありません。
`current_owner_observed_complete` と `current_write_verified` を分け、dedicated SAの実読取・fresh full7・migration readyは常にfalseです。
今回の結果を確認するまでは、先行の部分変更を全設定完了と扱いません。

ダミー検証： `python3 infra/gcp/tests/test_complete_codex_history_reader.py`。
source pins、default off、native version、etag/UID/role/policy漂流、already-complete、1SET、partial/UNKNOWN、期限、safe enumを検査します。実認証/APIはありません。

- [Table setIamPolicyのpath/body/default mask](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/setIamPolicy)
- [Table IAMとcustom role・version保持・etag](https://docs.cloud.google.com/bigquery/docs/control-access-to-resources-iam)
- [公式Python clientのTable policy操作](https://github.com/googleapis/python-bigquery/blob/main/google/cloud/bigquery/client.py)
