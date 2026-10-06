# User activity schema audit

履歴tableのmetadataと、legacy `timestamp` がある場合の4aggregateだけを確認するread-only CLIです。
row sample、user ID、timestamp sampleを出力しません。実行には別途data-access/query承認が必要です。

```sh
CREDENTIAL_FILE_LOCATION="$GOOGLE_APPLICATION_CREDENTIALS" \
  go run ./cmd/user-activity-schema-audit \
    development test-youtube-study-space asia-southeast2
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
