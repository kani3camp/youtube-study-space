# Firestore scheduled export（Gen1 / Node.js 22 準備）

この directory が development / production 共通の source 正本。今回は GitHub への source 正本化と migration 準備のみで、GCP deploy / 実 export / cloud resource 変更は行っていない。

## Recovery provenance

2026-10-03（JST）に、両環境の deploy 済み Cloud Function **version 5** を versionId 指定の read-only download で復元した。調査済み archive と metadata を用い、署名 URL や credential は保存しない。

- 元 archive は `index.js` / `package.json` のみ。**lockfile は存在しなかった**。当時 deploy 時の transitive dependency の exact version は不明で、今回の lockfile は新しい reproducible baseline。
- dev / prod の `index.js` は project ID / bucket URI の literal 以外同一。`package.json` は byte-identical。
- 最初の recovery commit に development の元 source をそのまま記録した。`tests/fixtures/version5/` はその byte-identical なテスト専用証跡で、deploy source / 第2の運用 implementation ではない。
- 元 dependencies: `@google-cloud/firestore: ^1.3.0` / `@google-cloud/pubsub: ^0.18.0`。Pub/Sub library は source で require / 使用されていなかった。

| 環境 | archive SHA-256 | index.js SHA-256 |
| --- | --- | --- |
| development | `19abaa897b3c1b017aa6b033c0404814c6433cc0bfa40029f76222efbd76b33c` | `71ed9e254006755734d6e890e58f70c8d08da2201855436062e60389ecc49568` |
| production | `7d0aaf2356da3e4e80ce426f8040b17726d74c7bcbc51b46ca2d2ca8c41c467e` | `7d90ac2fc9084094f0e2a705ad313a24447a59743df6f8c66783260f40cec6f7` |

両環境の元 package.json SHA-256: `ab298298c12c6b989b1624e0f8a83dc2bdd5ebd07628fbfadc70f9a590dee389`。

## Current behavior（変更しない契約）

`FirestoreAdminClient.exportDocuments` を呼び、database は `(default)`、`collectionIds` は **exactly** 以下の3つ。

```text
users
user-activities
order-history
```

`live-chat-history` / `work-segments` / `seats` / `member-seats` / `configs` / その他を追加しない。空配列による full database export も行わない。Pub/Sub payload / context の指定では対象を変更できない。

Function は export 開始 RPC の応答を待ち、返された Long Running Operation の name をログ出力して、その Operation を返す。`operation.promise()` / polling / 完了待機は追加しない。アプリケーションの retry / deduplication も追加しない。

**API Promise reject は console.error 後に undefined へ正常 resolve する現行挙動を維持する。** SDK の同期 throw は伝播する。TODO（別PR）: reject を再throwするか、失敗検知・再実行との関係を検討する。Function の成功ログだけでは export 完了・成功の証明にならない。

Scheduler / Pub/Sub / IAM / execution SA / bucket policy / retention は変更対象外。production の Go1.14 App Engine workload も別workloadで、今回触らない。[privacy runbook](../../../docs/privacy/raw-live-chat-archive-retirement-runbook.md) の retained domain data / raw archive の境界を維持する。

## Environment mapping / deployment definition

`environments.json` が runtime と deployment plan の共通設定。bucket の任意入力や payload 上書きを受け付けない。

| 設定 | development | production |
| --- | --- | --- |
| project ID | `test-youtube-study-space` | `youtube-study-space` |
| bucket | `firestore-backup-test-youtube-study-space` | `firestore-backup-youtube-study-space` |
| Function name | `firestoreCollectionsExport` | `firestoreExport` |
| region | `asia-southeast2` | `asia-northeast2` |
| Pub/Sub topic（同project） | `initiateFirestoreCollectionsExport` | `initiateFirestoreExport` |
| execution SA | `test-youtube-study-space@appspot.gserviceaccount.com` | `youtube-study-space@appspot.gserviceaccount.com` |
| memory | 256MB | 512MB |
| timeout | 60s | 120s |
| max instances | 1 | 1 |
| generation | Gen1 | Gen1 |
| entry point | `scheduledFirestoreExport` | `scheduledFirestoreExport` |
| current v5 runtime | `nodejs20` | `nodejs20` |
| next migration runtime | `nodejs22` | `nodejs22` |

新しい source は `YSS_EXPORT_ENVIRONMENT=development|production` と `YSS_EXPORT_PROJECT_ID=<expected-project>` の**両方を明示設定**する。`YSS_EXPORT_PROJECT_ID` は `environments.json` の project ID と一致しなければ module load 時に失敗する。`GCLOUD_PROJECT` / `GCP_PROJECT` はplatform依存のため必須入力にせず、存在する場合だけ追加の不一致検知に使う。project / bucket を独立に自由入力して環境を取り違える構造にしない。次 deploy では source と2つのYSS environment variableを同時に更新する必要がある。

## Dependency / Node.js 22 判断（2026-10-03）

1. 元1.3.0の `google-gax ^0.25.0` は native `grpc ^1.16.0` を含むため、そのまま Node.js 22 安全互換と判断しなかった。
2. 7.11.6を最小候補として評価。pure JavaScript gRPC と v1 Admin API は Node.js 22で利用できたが、npm audit が旧 uuid の [GHSA-w5hq-g745-h8pq](https://github.com/advisories/GHSA-w5hq-g745-h8pq) に起因する moderate 6件を検出。transitive override / 無条件の audit fix --force は使わない。
3. **8.7.1に exact pin**。npm公式metadataの engines は `node >=18`。`google-gax ^5.0.1` の pure JavaScript `@grpc/grpc-js` を使い、Node.js 22.22.0で実際のv1 Admin API / request / LRO開始surfaceのoffline testを通過。生成lockの npm audit は **0件**（確認日時点）。最新9.3.1まで上げる必要はないため追加major更新は避けた。
4. 元1.3.0と8.7.1の publish済み `FirestoreAdminClient` config の `ExportDocuments` は、いずれも non-idempotent / retry codes 空 / RPC timeout 60000ms。retry設定の追加・overrideはしない。元deploy lockがないため、SDK全内部差分がbyte-identicalとは主張しない。
5. 未使用Pub/Sub dependencyを削除。Gen1のPub/Sub trigger自体はplatformが呼び出し、sourceはpayloadを読まないため、実装behaviorには影響しない。testでもbaselineとの契約一致を確認。
6. packageの engines は22.x、CIもNode22。repository全体の `.node-version` / `.nvmrc` は変更しない。dev dependencyのBiomeをexact pinし、install時のscriptsは不要。依存更新は専用commitとして分離。

一次source: [Google client support policy](https://github.com/googleapis/google-cloud-node/blob/main/handwritten/firestore/README.md#supported-nodejs-versions)、[upstream changelog](https://github.com/googleapis/google-cloud-node/blob/main/handwritten/firestore/CHANGELOG.md)、[8.7.1 registry metadata](https://registry.npmjs.org/@google-cloud/firestore/8.7.1)、[元google-gax metadata](https://registry.npmjs.org/google-gax/0.25.0)。

Googleの [runtime support](https://docs.cloud.google.com/functions/docs/runtime-support) による Node.js 20 の deprecation は **2026-04-30**、decommission予定は **2026-10-30**。Gen1 Node.js 22 は対応runtimeで、decommission予定は2027-10-31（2026-10-03確認。実deploy前に再確認）。

## 2026-10-03 development deploy incidents

最初の Node.js 22 development deploy は Cloud Build の `function.js does not exist` で失敗した。Function 本体は更新されず、Gen1 / Node.js 20 / version 5 / ACTIVE のまま。

原因は runtime / dependency ではなく **source packaging**。当時の `.gcloudignore` は先頭の `*` で root directory 自体を除外し、gcloud 580.0.0 の Gen1 source ZIP 生成では後続の root file allowlist に到達できず **0 entry / 22 byte の空ZIP** を作成した。Buildpack は空の `/workspace` に package.json / index.js がないため fallback candidate の `function.js` を検査して失敗した。

修正:
- `*` の直後に **`!.`** を置き、root directory を先に再許可する。
- その後で6つの root fileだけを allowlist する。
- `tests/source-package.test.js` でこの順序、exact allowlist、package main の存在を固定する。

重要: `gcloud meta list-files-for-upload` は失敗時にも6filesを表示していたため、**実際の deploy ZIP の健全性を証明しない**。実deploy前には、使用する同じ gcloud CLI の packaging path で source ZIP を offline 生成し、root entry が exactly 6 files で空ZIPでないことを確認する。

2回目の development deploy は packaging gate を通過したが、build-time user-code load で `Project mismatch for development` となった。原因は `fromRuntime()` が `GCLOUD_PROJECT` / `GCP_PROJECT` の存在を必須前提にしていたこと。Googleは明示設定していないplatform environment variableへ依存しないことを推奨しており、このguardを `YSS_EXPORT_PROJECT_ID` の明示設定へ変更する。platform project aliasesは存在時だけ追加検証する。

## Local / CI verification

このpackage内でNode.js 22を選択して実行する。

```bash
node --version  # v22.x
npm ci --ignore-scripts
npm run check
npm test
npm ci --ignore-scripts
git diff --exit-code -- package.json package-lock.json
```

テストは credential / GCP API に依存せず、復元v5 fixtureと共通sourceの両方を契約検証する。実SDK testは架空credentialとRPC stubを使う。加えて source packaging test で `.gcloudignore` の root re-include `!.`、6file allowlist、package main の root 存在を検証する。実GCP export / runtime動作確認は次PRの自然実行で行う。

CIはpackage変更を `gcp_firestore_export` に分類し、Node22でlocked install / lint・format / 全unit・contract tests / lock再現性を検証し、CI Gateに含める。CI config変更時は既存方針どおり全groupを検証する。CIからdeployするstepはない。

## 次PR: development runtime migration runbook

順序: このsource-control PR → development runtime更新PR → development自然実行確認 → production更新PR・確認 → 安定後のTerraform import。すべて専用integration branchのstack/orderを明記し、最後にintegration branchからdevへ統合する。runtime migrationとTerraform importは別PR。

以下の **実deployは今回未実行**。developmentを対象とする次PRでcloud変更が明示承認された場合にのみ行う。

1. CIが成功したreview済みcommitを使用し、上のNode22 checksを実行する。read-only preflightで現行Gen1 / version / trigger / SA / memory / timeout / maxInstancesを確認し、表と不一致なら止めて差分を調査する。

   ```bash
   gcloud functions describe firestoreCollectionsExport      --project=test-youtube-study-space --region=asia-southeast2      --format='yaml(name,versionId,runtime,entryPoint,eventTrigger,serviceAccountEmail,availableMemoryMb,timeout,maxInstances,environmentVariables)'
   gcloud scheduler jobs describe scheduledFirestoreCollectionsExport      --project=test-youtube-study-space --location=asia-southeast2      --format='yaml(name,state,schedule,timeZone,pubsubTarget.topicName,retryConfig)'
   ```

2. package directoryで **コマンド生成だけ** を行う。`plan` はgcloudを実行しない。出力をeval / pipe to shellせず、内容を確認する。

   ```bash
   npm run plan -- --environment=development --project=test-youtube-study-space
   gcloud meta list-files-for-upload
   ```

   `.gcloudignore`によりruntimeのindex.js / config.js / environments.json / package.json / package-lock.jsonとignore設定だけをuploadする。tests / baseline fixture / README / plan / node_modulesをuploadしない。

   **注意:** `gcloud meta list-files-for-upload` だけでは acceptance にしない。2026-10-03 の失敗ではこの一覧が正常でも deploy ZIP は空だった。deploy前に、実際に使用する同じ gcloud CLI の source packaging処理を offline で実行し、一時ZIPの root entries が exactly以下6filesであることを確認する。

   ```text
   .gcloudignore
   index.js
   config.js
   environments.json
   package.json
   package-lock.json
   ```

   ZIP が空、余分なdirectory nesting、missing file、余分なfileのいずれかなら deployを止める。

3. 次PRで承認後、生成された具体的なdevelopment commandをoperatorが実行する。内容は次のとおり（source pathは現在のcheckoutを使う）。

   ```bash
   gcloud functions deploy firestoreCollectionsExport      --project=test-youtube-study-space --no-gen2 --runtime=nodejs22      --entry-point=scheduledFirestoreExport --region=asia-southeast2      --trigger-topic=initiateFirestoreCollectionsExport      --service-account=test-youtube-study-space@appspot.gserviceaccount.com      --memory=256MB --timeout=60s --max-instances=1 --no-retry      --update-env-vars=YSS_EXPORT_ENVIRONMENT=development,YSS_EXPORT_PROJECT_ID=test-youtube-study-space --source=.
   ```

   API enable / IAM追加を要求されたら、このruntime更新と別scopeとして止めて調査する。Scheduler / topic / SA / retentionを変更しない。

4. describeを再実行し、runtime=`nodejs22`、environment variable、Gen1、trigger・SA・resource limitsを確認。preflightとの意図的差分はsource / dependencies / runtime / YSS_EXPORT_ENVIRONMENTのみ。
5. Schedulerを手動runしたりPub/Sub publishせず、次の自然実行を待つ。復元metadataではdevelopmentのscheduleは `0 0 * * *` / `Asia/Tokyo`（次deployのpreflightがauthority）。以下はread-only確認例。

   ```bash
   gcloud functions logs read firestoreCollectionsExport      --project=test-youtube-study-space --region=asia-southeast2 --limit=50
   gcloud firestore operations list --project=test-youtube-study-space --database='(default)'
   gcloud storage ls gs://firestore-backup-test-youtube-study-space/
   ```

   新規 `Operation Name` と同じoperationをread-only describeし、完了・errorなし・metadataの3 collectionIdsを確認する。新規snapshot全体とmetadataを確認し、3groupsのみ、`kind_live-chat-history`や他groupがないことを確認。ユーザーdataの内容をPRログへコピーしない。Functionのresolveだけで完了成功と判定しない。
6. Node20のdecommission後はNode20への再deployをrollback手段としない。問題時はproductionへ進まず、Node22上でreview済みsource修正を行う。production更新・Terraform importは別途承認されたPRで行う。

## Production plan guard

productionのplan生成には、明示environment / projectに加えて両方のguardが必要。

```bash
npm run plan -- --environment=production --project=youtube-study-space   --allow-production --confirm=production:youtube-study-space:firestoreExport
```

これはコマンド生成のみで、cloud変更の承認を意味しない。guardはplan toolの誤操作防止で、gcloud自体を直接実行するoperatorの権限を制限するものではない。
