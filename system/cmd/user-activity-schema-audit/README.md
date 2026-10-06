# User activity schema audit

履歴tableのmetadataと、legacy `timestamp` がある場合の4aggregateだけを確認するread-only CLIです。
row sample、user ID、timestamp sampleを出力しません。実行には別途data-access/query承認が必要です。

```sh
CREDENTIAL_FILE_LOCATION="$GOOGLE_APPLICATION_CREDENTIALS" \
  go run ./cmd/user-activity-schema-audit \
    development test-youtube-study-space asia-southeast2 1073741824
```

`.env`は読みません。credential fileの場所とBigQuery locationを明示します。
job/tableのprojectは次のexact環境対応を検証した引数から設定します。

| Environment | Explicit project |
| --- | --- |
| development | `test-youtube-study-space` |
| production | `youtube-study-space` |

WIFの`external_account` JSONはresource projectの`project_id`を持たない場合があります。
credential内のprojectが空でも、検証済みexplicit targetを使います。projectが存在してtargetと
違う場合は拒否します。`GOOGLE_CLOUD_PROJECT`や`GCLOUD_PROJECT`等からtargetを推測しません。
credential metadataのprojectはidentity/trustの証明ではありません。dedicated identity、exact
table/project権限、WIF cloud trustとprotected Environmentは別途検証が必要です。

CLIにproduction targetが存在しても、専用GitHub audit workflowはdevelopment-only、default-offです。
raw JSONとstderrはprivate ephemeral outputとして扱い、public logs/artifactsへ保存しません。
workflowの公開Summaryはzero/non-zeroだけです。source/test PASSはquery実行承認を兼ねません。

credentialless regression testsは合成WIF JSONを使います。token fileは存在せず、token交換や
BigQuery queryを行わずに、projectless credentialの読み取りと起動準備を検証します。

## Query budget and dry-run boundary

`maximum-bytes-billed`は必須のpositive decimal integerで、hard ceilingは1 GiB
(`1073741824` bytes)です。未指定・zero・negative・overflow・上限超過はcredential読取前に
拒否します。workflowもこの上限を明示し、query requestの`maximumBytesBilled`へ設定します。
これは次のcost承認候補であり、実queryの承認ではありません。低い明示budgetも指定できます。
上限超過はSTOPし、budget引上げやquery retryを自動で行いません。canonical schemaならquery0です。

[BigQuery公式cost controls](https://docs.cloud.google.com/bigquery/docs/best-practices-costs)
に従い、実行前にmetadata、billing model、region単価、quotaをprivateで確認します。
`maximumBytesBilled`はon-demand queryのbytes上限で、capacity/slot料金全体のmoney capでは
ありません。tableの`numBytes`/`numRows`は現在のbuffer/変更を含む正確なquery costの証明ではなく、
実4COUNTIF SQLのdry-runでfresh見積もりを取る案です。

[公式dry-run API](https://docs.cloud.google.com/bigquery/docs/running-queries#dry-run)
は`dryRun=true`でvalidation/bytes estimateだけを返し、query slotsを使わずquery chargeなしです。
SDKでは`Query.Run`+`LastStatus`を使い、`Read`/`Status`/`Wait`を呼びません。
ただしauthenticated cloud requestであり、現時点で未承認・未実行です。Aのidentity/限定grant/
Environment準備承認は、token交換やdry-runを含みません。既存operatorでdry-run1回を許可する
場合もtarget/同一SQL/認証経路/output経路を別途承認し、public出力しません。
Row-level securityでmaskedなtableはestimate0になり得るため、zeroを無料・安全の根拠にしません。
Workflowはdry-runを自動追加せず、real aggregate operation1回だけという契約を保持します。

Credentialless regressionはpinned SDKのHTTP transportを合成responseで置換し、
exact4COUNTIF/target/location/budgetのwire request、canonical query0、budget error後query retry0、
invalid budget時metadata/query0を確認します。実cloudへの接続はありません。
