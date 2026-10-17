# GCP Terraform

development履歴tableのまとまった判断packet・本人の先行準備・本番への依存は
[`docs/development-history-ready-packet.md`](docs/development-history-ready-packet.md)を参照してください。

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope（2026-10-17）

現行運用は固定baselineのfull-root監査planと、承認済みimport/no-op・State-only saved applyを扱う [`migration-plan` / `migration-apply`](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)です。
入口は既存 `ci.yml` の本人初回 `workflow_dispatch`、専用integration branchです。
実移行・検証の記録にはH5（`d79d1cc4`）を使用しています。最新の運用Sourceと最終runは[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照してください。H4（`93121ab6`）の[#1364](https://github.com/kani3camp/youtube-study-space/pull/1364)でnative10 caseの上限2並列化とrouting/回帰を追加し、H5の[#1366](https://github.com/kani3camp/youtube-study-space/pull/1366)でnative runnerと回帰テストを修正しました。#1366は30 checks（12 success/18 skipped）・独立review後に統合済みで、runtime controller5ファイル・認証workflow・HCLはH3/H4と同一です。[CodeQL #1362](https://github.com/kani3camp/youtube-study-space/pull/1362)の実証はH3 pushの[run39001594887](https://github.com/kani3camp/youtube-study-space/actions/runs/39001594887)で3言語解析・SARIF受付成功として保持します。
全4 Terraform EnvironmentはRequired reviewersなし・admin bypass=falseで、plan/apply identityを分離します。
旧issuer、期限付きwindow、毎回のEnvironment人間承認は現行経路へ戻しません。`dev`へのmerge・直接push・auto-mergeは行わず、最終mergeはユーザー本人だけが行います。

| 環境 / 対象 | 実際に確認した範囲 | 未完了の範囲 |
| --- | --- | --- |
| Development | [run38945893535](https://github.com/kani3camp/youtube-study-space/actions/runs/38945893535)の両identityで31所有・import0・no-op31・raw drift0、State/version/lockと独立review PASS。DevExit後のfresh自然回帰も5 stage・finding0、Lambda3/6で1,338実行・Errors/Throttles0、独立review PASS | quiet3件とcutoff後の日次処理は未観測。未来の最終no-op後healthと扱わない。original history final receipt欠落は保持 |
| Production backend / CI foundation | 既存共通S3 bucketを再利用し、AWS backend roles2・inline policies2、GCP CI resources7・個別policy5へのmember追加8/削除0、Prod2 Environmentへの設定7種ずつを実readback・独立review済み | 新bucket・長期key・業務resource変更なし。正規CIでのState/ownership検証は別の実行結果で判定する |
| Production State / ownership | H2の[初期化run38961710157](https://github.com/kani3camp/youtube-study-space/actions/runs/38961710157)成功。H3で初版State0のversion GET/checksum・controls9・Dev不変を直接監査／独立PASS。H3の[run39005902978](https://github.com/kani3camp/youtube-study-space/actions/runs/39005902978)まで既存9件のownershipとpost no-opを検証済み | H4の[第10波run39014888191](https://github.com/kani3camp/youtube-study-space/actions/runs/39014888191)は認証付きPlan/Apply・post10成功後、native runner回帰の静的CIとGateで失敗。State10/直前9のversion固定GET・厳密差分照合は独立PASS。元runの静的CI失敗を保持します。State10照合後にH5の[第11波run39021164334](https://github.com/kani3camp/youtube-study-space/actions/runs/39021164334)を1回発行しました。認証付きPlan/Applyは成功しましたが、native runnerのtimeout/grandchild回帰で静的CIとGateが失敗しました。State11/直前10のversion固定GET・厳密差分照合も独立PASSで、直前10の本文/HEADは第10波の独立確認済みcurrentと完全一致します。元runの静的CI失敗を保持し、第12波は未発行・再送0です。全30件・両identityの最終full-root no-op・最終State/Production回帰は未完了。Infrastructure ReadyはHOLD |

Production rootの既定選択は0です。Developmentの既存基礎graphと各opt-inはsource定義に従います。
実運用は両環境ともreview済みprivate baselineで既存所有をすべて選択し、候補を1件ずつ追加します。
未選択の既定値を既存Stateへ適用したり、既存所有をbaselineから落としたりしません。
Stateの初回作成はProductionの正規不在・manifest0だけに許す固定outputs2件の例外で、一般resource createは許可しません。

最新の実行結果・source/run・残件は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)、
[Production runbook](docs/production-readiness.md#current-execution-boundary--2026-10-17)、
[Infrastructure / MyPage ReadyGate](docs/phase2-approval-and-readiness.md#development--production--mypage-readygate)に集約します。
TerraformによるIaC拡張の準備と、MyPage固有resourceのprovisioning・Production releaseは別判定です。
新規resourceは専用normal-change PRでtype/schema、create/update専用の実行経路・検証契約、exact IAM/API、費用・rollbackを整えます。HCLや権限の追加だけでは現migration経路から作成できず、import-only契約は維持します。
現行CIのtrustは専用integration refに固定しています。最終devマージ後もこの運用refを必要な間保持し、ref移行・branch削除はworkflow、Environment、cloud trustを整合させる別reviewで判断します。

今回、過去raw-chat/混在backup削除、通知修復、CloudTrail継続課金、MyPage provisioningは行いません。
[recovery運用 #1165](https://github.com/kani3camp/youtube-study-space/issues/1165#issuecomment-6223368027)、
[durable audit #1166](https://github.com/kani3camp/youtube-study-space/issues/1166#issuecomment-6223368703)、
[Privacy cleanup #983](https://github.com/kani3camp/youtube-study-space/issues/983#issuecomment-6223369320)はすべてOPEN・Acceptance未達で、今回のexisting ownership移行のblockerにはしません。
MyPageのProduction release前の運用判断とPrivacyの独立laneへ残します。
CodeQLのintegration refには未解決finding6件（critical1/high3/medium2）があり、dismiss/suppressしません。解析・SARIF受付成功を脆弱性0と扱わず、[別対応の境界](docs/development-history-ready-packet.md#codeql-and-mandatory-oidc-claims)を保持します。

## 旧protocolの経緯（現行の実行条件ではない）

<details>
<summary>2026-10-16以前のhistory adoption、issuer、専用windowの記録</summary>

以下は当時の記録です。旧gateのfalse/未完・人間承認の記述を現在のmigration運用へ適用しません。

2026-10-16の本人初回dispatch・Required reviewers不要の定常経路は、[現行Plan/Apply契約](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)を参照してください。以下の毎回Environment承認・旧issuer/windowは履歴の記述であり、新経路に適用しません。Productionの実移行gateはDevelopment Exitまで閉じます。

Phase 1のscaffold / S3 state / protected CIと、developmentの主要11 resourceおよび `user-activity-history` のownership移行は完了しています。2026-10-15時点の認証付き実証は[fresh current-state run 38617630530](https://github.com/kani3camp/youtube-study-space/actions/runs/38617630530)の `import0 / no-op12 / drift0 / change0`（観測 `2026-10-14T22:18:59Z`）です。観測時点の結果として記録し、次waveではfresh Stateを再照合します。後続はIssue #1191を入口に、runtime WIF / 必要な個別IAM / 明示Own APIとdevelopment exitを完了してからproduction migrationへ進みます。

dedicated development aggregate auditの実行記録・revoke手順と、development→production→MyPage releaseの証拠条件は
[`docs/phase2-approval-and-readiness.md`](docs/phase2-approval-and-readiness.md) を参照してください。
source準備や過去のno-op記録を、fresh実環境検証・実行承認・Infrastructure Readyと扱いません。
history tableは既に12件目としてStateへ追加済みです。legacy列の旧監査記録と修復条件は
[`docs/user-activity-history-canonicalization.md`](docs/user-activity-history-canonicalization.md)を参照し、
fresh `tables.get`と現Stateを照合します。canonical8列なら再DROP・再importを行いません。
original history applyのfinal receiptは失われており、current-state証跡はその履歴を遡及証明しません。

Notion Current Canonの[2026-10-15 条件付き包括承認案A](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)
の範囲内は、作業ごとのChat再承認を求めず進めます。2026-10-16のCurrent Canon §10を優先し、
全4 Terraform EnvironmentはRequired reviewersなしで運用します。本人ID54093651 / kani3campの
初回workflow_dispatchを認証前に検証し、integration branch制限・必要CI・OIDC/WIF trustと
技術的STOP条件を維持します。AWS CLI/gcloudのログインと最終devマージは本人が行います。
古い文書の別Chat承認という記述は案Aの対象範囲で更新されましたが、fresh inventory、exact SHA、
import-only、same-SHA re-plan、post no-op等の技術的STOP条件は変わりません。
`dev`へのmerge、直接push、auto-mergeはCodexやbotでは行いません。

通常のmigration pathでは以下を行いません。bounded normal-change / state-only例外は、別validator・別approvalで明示的に隔離します。

- 未承認のproduction resource import / apply
- 想定外のworkload create / update / delete / replacement
- manual Scheduler / Pub/Sub / Firestore export trigger
- importと同時のAPI enable / disable
- broad IAM role追加
- Service Account JSON keyの作成
- MyPage resourceのprovisioning

Firestore export FunctionのNode.js 22自然実行E2Eはdevelopment / productionともPASSし、#1173はcompletedでclose済みです。developmentのPub/Sub topic / Cloud Scheduler / Gen1 Functionもそれぞれprotected import-only waveでownership移行済みで、当時のpost-planはno-op11 / drift0です。generated subscription / build artifact / source deploymentはownership外を維持します。

runtime WIF/APIの後続source-only配線と累積graph validatorは
[`docs/runtime-wif-api-ownership.md`](docs/runtime-wif-api-ownership.md#2026-10-10-後続-source-package-protected-選択と累積-graph)を参照してください。
既存bindingを保つ追加入力には、同じEnvironment Secret内の可逆圧縮形式も用意しています。
[`圧縮・private handoffの契約`](docs/runtime-wif-api-ownership.md#同じ追加入力-secret-に収める可逆圧縮)によるM14 consumer `bea636351f3d315d5b2012e2d5cc2f9c866d354f`の配布入力とissuerは、`2026-10-15T18:43:51Z`で初回消費期限を過ぎました。protected dispatchは行わず、本人への追加Secret2か所の保存依頼を撤回し、4つのruntime実行ゲートを閉じています。旧入力は履歴だけとして保持し、期限や観測時刻を更新しません。
Codex Cloud の既存調査用 keys / scoped AWS ReadOnlyRole / 専用 GCP WIF の再利用手順と現在の metadata gap は
[read-only 認証 runbook](docs/runtime-wif-api-ownership.md#codex-cloud-の調査用-read-only-認証経路)に記録しています。
既存plan/apply Environmentそれぞれでruntime六種metadataを読む `runtime-metadata-read` source routeは
別default-off gateと最大8時間windowを持ち、Terraform/State/lock/receipt/artifactを扱いません。
[source契約](docs/runtime-wif-api-ownership.md#protected-runtime-metadata-read-の-source-routedefault-off未実行)を参照してください。
history importは成功し、過去の独立protected planでimport0/no-op12を確認済みですが、original applyのfinal receiptは失敗し、当時のprivate captureは失われています。current-state bootstrap契約（packet v3）は12件を新しい起点として扱い、失われた歴史証拠を復元しません。旧issuer source S18とM14入力は期限切れの履歴として残し、採用可能なfresh証拠へ読み替えません。current-state root-ready / runtime-plan / split-input / development applyはfalse、metadata-read / authority / current-state-read / productionも閉鎖を維持します。emitter / history-post-noop / 管理済み12件のownership、M13のPool4属性修正、固定診断、全guardとcatalogは保持しています。次は新source、承認する固定4MiB上限（外部の実サイズは未測定）、別途fresh確認した既知writer静止を結ぶ新public safety profileを使い、同じ保護job内のfresh HEAD検証を経てcurrent-state baseline planを発行する経路です。新しい発行・issuer登録・fresh inventory・初回Poolのadmission/import成功はまだ成立していません。baseline発行では追加reviewed-inputs Secretをstageしないため、今は本人による2か所の更新は不要です。既存binding Secretは保持し、実shape/metadata一致はrunnerで検証します。[期限切れ後の閉鎖と再発行条件](docs/runtime-wif-api-ownership.md#2026-10-15-m14配布入力の期限切れと実行枠閉鎖)を参照してください。

その後の[current-state baseline CI #878](https://github.com/kani3camp/youtube-study-space/actions/runs/38828914021)は、本人のEnvironment承認後にStateの事前検証・init・saved planまで進み、Terraform exit codeは0でしたが、sanitizerと事後no-op検証が失敗しました。原因のfield・実action件数・事後State/lockは未確認で、cleanup成功、apply skipped、artifact0です。4つのruntime gateは閉じたまま、成功issuerは未発行です。後続sourceはこのbaseline条件だけに値を含まない固定診断を追加し、既存guardや失敗判定を保持します。今は本人の追加設定操作は不要です。[実測と診断の範囲](docs/runtime-wif-api-ownership.md#2026-10-16-current-state-baseline失敗と限定診断)を参照してください。

続く[baseline CI #881](https://github.com/kani3camp/youtube-study-space/actions/runs/38832507626)は、本人承認後の固定診断でno-op12 / drift1を観測し、sanitizerはSTOPしました。State本文/HEAD不変など5つの独立safety checkはPASSでしたが、driftの対象・fieldは未確認で、成功issuerは未発行です。次のsourceはbaseline限定で対象の固定aliasと公開schema由来のtop-level field codeだけを診断へ追加します。drift0の条件を保ち、refresh applyや旧runの再試行は行いません。追加の本人操作は今は不要です。[観測と追加診断の境界](docs/runtime-wif-api-ownership.md#2026-10-16-baseline-drift1の観測と対象fieldの限定診断)を参照してください。

[CI #885](https://github.com/kani3camp/youtube-study-space/actions/runs/38836240227)では、drift1がhistory Tableの`etag` / `last_modified_time`だけと分かり、strict guardは引き続きSTOPしました。この2項目をStateへ同期する[実行不可の設計・pure prototype](docs/history-observation-sync-prototype.md)を用意しています。refresh-onlyの実保存planと2leafだけの永続化は未証明で、成功結果も実行許可を返しません。既存workflow・gateは変更せず、State書込の追加許可や本人操作を今は求めません。

M20では[専用観測同期protocol](docs/history-observation-sync.md)をsource-onlyで導入し、新gateを既定falseとしました。既存通常guard・旧prototypeは不変です。完全隔離ダミーのserializer/check-set根拠とfull12検証を分け、same-SHAのpreview QA後、別apply Environmentで2属性と検査結果内容不変を明示した本人承認を必要とします。実State変更・新issuer・移行完了は未成立で、今は本人操作を求めません。

M21では専用source windowをUTC `2026-10-16T04:29:31Z` → `2026-10-17T04:29:31Z` に固定しました。その後の[preview CI #893](https://github.com/kani3camp/youtube-study-space/actions/runs/38856804355)は、本人のplan Environment承認後に認証・private binding・metadata・事前State・initまで進み、OBSERVATIONでSTOPしました。通常plan終了コード、show/JSON読取、strict観測検証のどこで失敗したかは未確定で、candidate未観測、適用CLIの予約/呼出0、apply skipped、cleanup/post-auth成功、artifact0です。事後State/lock不変や同期成功の証拠とは扱いません。専用2gateをfalse・時刻を空へ閉じ、次回向けの値を含まない失敗診断だけを追加します。旧4runtime gate・production・全guardは不変で、旧run再試行や旧window延長を行わず、本人の追加操作は今は不要です。[停止と診断の境界](docs/history-observation-sync.md)を参照してください。

</details>

## Directory

```text
infra/gcp/
├── .terraform-version
├── environments/
│   ├── dev/
│   │   ├── main.tf
│   │   └── backend.hcl.example
│   └── prod/
│       ├── main.tf
│       └── backend.hcl.example
└── modules/
    └── README.md
```

dev / prodは別root moduleです。Terraform workspaceで環境を切り替えません。

## Version policy

- Terraform CLI: `.terraform-version` と各rootの `required_version` を一致させる
- Google provider: 各rootの `required_providers` と `.terraform.lock.hcl` を正本にする
- provider lockはTerraform operator / CIで使用する `linux_amd64` / `darwin_arm64` のchecksumを事前生成する
- version更新は通常のdependency変更としてPRでreviewする
- Notionにはpatch versionを複製しない

## Local validation

GitHubと別に管理権限でログイン済みの端末で必要な設定だけを取得する場合は、
[`設定を読むツールの手順`](docs/runtime-operator-metadata-collector.md)を参照してください。
この設定読取ツールは権限を追加せず、取得結果や実データを公開しません。

既存CIに不足するRuntime読取権限の設定候補は、
[`一度だけの設定をレビューするための準備物`](docs/runtime-reader-bootstrap.md)にまとめています。
既定は認証・HTTP・変更なしのoffline検査です。通常GCPログイン経由の作業は本人が継続的に許可しており、具体的な対象・差分・実行条件はレビュー済みpacketで限定します。

設定後の両CI identityによるRuntime6種の前後readは[run38712857940](https://github.com/kani3camp/youtube-study-space/actions/runs/38712857940)でPASSとなり、使用済みmetadata実行枠を閉鎖しました。
fresh full7 metadata、root admission、import準備完了は別条件です。[結果と範囲](docs/runtime-wif-api-ownership.md)を参照してください。

既存の専用Codex dev metadata SAへ同じRuntime読取と保持history Tableのmetadata読取だけを追加する別の準備物は、
[`専用の設定読取SA向けの限定差分`](docs/codex-metadata-reader-bootstrap.md)を参照してください。既定はofflineで、先行設定を再実行せず、全policyのIAM etagを必須にします。
その先行結果で残ったTable設定だけを扱う別の準備物は、[`history Tableだけの完了候補`](docs/codex-history-reader-completion.md)に記載しています。既存3変更を再実行せず、Table native version 1と最大1試行を要求します。

専用SAでの読取を行う任意の[Codex向けcollector](docs/codex-runtime-metadata-collector.md)も既定はofflineです。
レビュー済みprivate pinsと短い実行枠を必要とし、既存operator用CLIやprotected CIの実行条件は変更しません。

AWS設定読取ツールのCLI投影テストは、固定6関数以外の値を保存しないことを実際のJMESPath parserで検査します。
本体の依存は標準ライブラリーのみです。テスト専用のhash固定依存を隔離環境へ入れて実行します。
通常PRのGCP Terraform CIも同じrequirementsから導入し、省略せず実行します。

```sh
GCP_TEST_VENV="$(mktemp -d /tmp/gcp-contract-tests-XXXXXX)"
python3 -m venv "$GCP_TEST_VENV"
"$GCP_TEST_VENV/bin/python" -m pip install --no-cache-dir --require-hashes --only-binary=:all: -r infra/gcp/tests/requirements.txt
"$GCP_TEST_VENV/bin/python" infra/gcp/tests/test_collect_aws_runtime_versioned_config.py
"$GCP_TEST_VENV/bin/python" infra/gcp/tests/test_aws_runtime_list_functions_projection.py
```

credentialやbackendを使わない構文・provider validation:

```bash
terraform fmt -check -recursive infra/gcp

for env in dev prod; do
  (
    cd "infra/gcp/environments/$env"
    terraform init -backend=false -lockfile=readonly
    terraform validate
  )
done
```

`terraform init -lockfile=readonly` を通常validationに使い、provider lockの変更を暗黙に許可しません。dependency更新時は `terraform providers lock -platform=linux_amd64 -platform=darwin_arm64` でdev / prod両rootのlockを更新し、差分をreviewします。

## Remote state bootstrap

State infrastructureはTerraform本体の外側にある one-time bootstrap exception とします。

### Architecture

Issue #1155以降の検討を踏まえ、remote stateは **AWS S3の個人開発共通Terraform state control plane** に置きます。管理対象がGCPでもbackendを同じcloudへ置く必要はありません。

原則:

- 個人開発共通のstate bucketは原則1つ
- product / environmentごとにstate keyを分離する
- YouTube Study Spaceは `youtube-study-space/dev/terraform.tfstate` と `youtube-study-space/prod/terraform.tfstate`
- Terraform workspaceでdev / prodを切り替えない
- S3 backendの `use_lockfile = true` を使い、DynamoDB lockは新規採用しない
- S3 Bucket Versioningを有効化する
- S3 Block Public Accessを全面有効化する
- server-side encryptionを有効化する
- state bucketへTerraform state / lock以外のbusiness dataやartifactを置かない
- product / environmentごとにbackend IAM roleを分離し、対象state keyとlock keyだけへ最小権限を付与する
- state file本体には原則 `GetObject` / `PutObject`、lock fileには `GetObject` / `PutObject` / `DeleteObject` を許可し、bucket listも対象prefixへ制限する
- backend roleへAWS workload用の広い権限を付与しない
- 長期AWS access key / secret keyを作らない

Issue #1161で **AWS Organizations配下の専用Terraform/state control-plane member account** と `ap-northeast-1`（東京）のS3 backendをbootstrap済みです。development / production workload accountへ共通stateを置かず、production workload accountがOrganizations management / payerを兼務している構成の見直しは別scopeとします。実account ID / bucket名 / role ARN等はpublic repositoryへ保存しません。

### Public repository CI boundary

このrepositoryはpublicのため、Terraform CIを次の2段階に分離します。

**通常PR CI（credentialなし）**

- `terraform fmt`
- `terraform init -backend=false -lockfile=readonly`
- `terraform validate`
- provider lock completeness
- static / security checks

forkを含む通常PRへAWS / GCP credentialを渡しません。

**現行の認証付きplan / apply**

- GitHub OIDC → AWSの短期credentialでS3 backendへアクセスする
- GitHub OIDC / Workload Identity Federation → GCPの短期credentialでtarget workload projectへアクセスする
- long-lived AWS key / Google Service Account JSON keyをGitHub Secretsへ保存しない
- cloud側trustはrepositoryだけでなくtrusted ref / GitHub Environment / workflow等へ可能な限り限定する
- 全4 Terraform Environmentは本人初回workflow_dispatch限定・Required reviewersなし。Productionの実行はDevelopment Exitと独立レビュー後のreadiness gateを必須とする
- plan用identityとapply用identityを分離し、それぞれのEnvironmentから専用least-privilege identityを使う
- full saved plan file / raw stateをpublic Actions artifactへuploadしない
- `terraform show -json` 等のraw sensitive outputをpublic logへ出さない
- public log / PR commentへ出すのはsanitized summaryを原則とする
- application secret valueはTerraformへ極力流さず、Secret Manager等のsecret storeとwrite-only / ephemeralな経路を優先する

外部forkのTerraformコードへcredential付きplanを自動実行しません。authenticated planをPR前に行う場合も、trusted same-repository commitを明示的なgate後に実行する設計とします。

### 旧protocol履歴：Authenticated workflow source guard

以下からsingle history plan receiptまでの旧mode/gate/毎回Environment承認は、当時の履歴です。
現行の実行には[定常Plan/Apply](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)を使い、
旧windowの再発行や完了済みhistoryの再importは行いません。

Issue #1162のsource-control側guardrailとして、default branchにも存在する `.github/workflows/ci.yml` の `workflow_dispatch` を入口にし、同一commitの `.github/workflows/gcp-terraform-authenticated.yml` reusable workflowを呼び出します。専用workflowを直接 `workflow_dispatch` にしないのは、移行中はこの新規workflow fileがdefault branch (`dev`) に存在せず、GitHubのmanual dispatch入口として成立しないためです。

development planとapplyは独立gateを持ちます。planの有効化はtrust構築後、applyの有効化はplan smoke / negative test PASS後の別変更です。

- `DEV_AUTHENTICATED_TERRAFORM_ENABLED=true`（development trust構築後のplan smokeのみ）
- `DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=false`（historyのplan-only waveとclosureで維持。再有効化は未実施security検証も含めた別review / approvalが必要）
- `PROD_AUTHENTICATED_TERRAFORM_ENABLED=false`
- development applyは別Environmentの承認と同一SHAの再plan / import-only検証を引き続き必須とする。gate有効化だけではapplyを実行せず、workload importは後続waveで扱う
- authenticated executionの入口は既存 `ci.yml` の `workflow_dispatch` のみ。`terraform_authenticated=true` を明示したrunだけreusable workflowを呼ぶ
- 任意commit SHAはinputで受け取らない
- PR headへcredentialを渡さない
- integration期間中は `feature/gcp-terraform-iac` 以外のrefを拒否し、callerも `ci.yml@refs/heads/feature/gcp-terraform-iac` に固定する
- repository nameに加えてimmutable repository / owner IDもpreflightで確認する
- authenticated jobだけに `id-token: write` を付ける
- external Actionsはfull commit SHAへpinする
- AWS assumed-role ID / role名、GCP project number / SA名も認証Actionより前にmaskする。Environmentには既存5 secretに加えて `AWS_TERRAFORM_BACKEND_ROLE_ID` を設定する
- backendのworkspace discovery prefixは対象environment配下に固定し、`TF_WORKSPACE=default` を使用する
- development planは空resource graphでもGCP WIF token交換とplan SA impersonationを強制し、project metadataのharmless readで認証を証明する
- `mode=plan`のidentity smokeは`plan-read-only`を使用し、同一SHA OIDC claim、plan identity、exact dev state read、dev project/Function metadata GETとdevの必要GET・mutation permission不在だけを確認する。state本体PUT、apply role/SA credential発行試験、prod / 他productのread/listとprod permission検査は実行せず、Summaryに`SKIPPED`と`Full security gate: NOT VERIFIED`を記録する。Terraform native dev `.tflock`の通常PUT/GET/DELETEはplanに必要な一時操作として別に扱う
- historyの`mode=apply` plan jobは専用`apply-read-only` checkを使う。`plan-read-only`のMODE=plan/safety admissionへ流用せず、exact OIDC、dev GCP/STS、state HEAD/GET/HEADとlock LISTを確認する。apply jobのFunction checkもこのmodeではdev-only。条件付きPut、wrong-role STS、prod/他product、別SAのnegative requestをapply経路から呼ばない
- full negative smokeは別gate `DEV_TERRAFORM_SECURITY_PROBE_ENABLED=false`の`security-probe` job/modeに限定する。fresh HEADと異なるrandom ETagの`If-Match`による条件付きPutObject拒否は403のみ合格で、前後version/body/lock差分や412/404/409はSTOP。条件付きrequest/headerの結果だけでrole/bucket/session/SCP全体を証明しない。別のlive承認・private policy review・probe結果がapply gateを開く前に必要
- raw init / plan / apply出力はpublic logへ流さない
- saved planはrunner一時領域だけで扱い、artifact / cacheへ保存しない
- public outputは `.github/scripts/terraform_plan_summary.py` が生成するresource address / action count中心のsanitized summaryだけ
- import移行期はcreate / update / delete / replacement / driftをstopする
- plan jobとapply jobでsaved planを渡さず、apply jobは同じ `github.sha` から再planし、sanitized projectionが一致した場合だけ同一job内のplanをapplyする

historyの次applyについて、現行`mode=apply`はapply gate=falseでpreflight停止します。
probe gateもfalseで、source準備だけではfull security gateを満たしません。
live probeとrole/bucket/session/SCPのprivate reviewは別承認で行い、plan-only成功を
full security gate PASSへ読み替えません。

developmentのGitHub Environment / branch trust、AWS GitHub OIDC backend role、GCP GitHub WIF / Terraform Service Accountは#1162で構築・実測済みです。通常PRはcredentiallessのまま、authenticated executionはtrusted integration refと独立Environment approvalへ限定します。production側のbackend / trust / identityは未開始で、#1191の別approval境界です。

production backendは未bootstrapのため、production authenticated plan / applyはbackend準備完了まで有効化しません。

### 旧protocol履歴：Single development history plan receipt

history executionは既存protected identityとsecretを使い、routine Terraform cost blockingのwaiverを安全条件から分離する。新しいbilling API、料金照合、月次予約を実行条件にしない。過去の見積・台帳は履歴であり、実請求額はunknownのまま。新resourceやbulk processingの承認はこのwaiverに含めない。

caller input `terraform_history_plan_safety_evidence`（reusableでは`history_plan_safety_evidence`）は以下のキーだけを許可する。旧`terraform_plan_cost_evidence`をhistory executionに渡すと認証前にSTOPする。state/metadata、bucket/account/role/SA名、privateな証拠本文を公開inputへ貼らない。

| Key | Required evidence |
| --- | --- |
| `schema_version` | 整数`1` |
| `git_sha` | review・公開された実行対象の完全なSHA |
| `issued_utc`, `expires_utc` | `YYYY-MM-DDTHH:MM:SSZ`。発行済み・期限内、発行後最大24時間。planとpost safety readの余裕を確保する |
| `max_state_bytes` | 整数`1..4194304`の承認上限で、実測サイズではない。今回のfresh baselineは固定`4194304`を選び、実サイズは外部未測定とする |
| `state_writers_quiescent` | 対象state/workspace prefixの並行Terraform writerがないと確認した場合だけ`true`。BigQueryの通常writer停止は要求しない |

認証前に0600/exclusiveの`history-plan-admission.json`を生成する。AWS認証後、GCP認証やstate GET/native backend initの前に、既存Environment secretのRoleIdとSTS `UserId` prefix、exact accountを照合する。history plan/applyの両jobで必須。追加grantやnegative probeを使わない。

今回のcurrent-state baseline再発行では`max_state_bytes=4194304`（既存`MAX_BYTES`の4MiB）を承認上限として指定する。
別の外部HEAD jobやbucket locatorによる事前実測は要件にしない。exact実行SHA、新しいUTC発行時刻・最大24時間の期限、
別途fresh確認した既知writer静止は引き続き必要で、上限を選んだことから実サイズやwriter静止を推定しない。
本人のEnvironment承認後、同じprotected jobのidentity検査は最初のState本文GETより前にfresh HEADで
`0 < ContentLength <= 4194304`、有効なVersionId / ETag、STANDARD・既知暗号化等を検査し、bounded GET後に全HEAD応答との一致を再確認する。
後続post-noop snapshotもprofile上限を使ったHEAD→bounded GET→全HEAD再照合をinit/plan前に行う。
この経路ではprofileとidentityの既存固定上限が**同じ4194304のため**一致する。4MiB未満のprofileがidentity段階の上限も下げるとは扱わない。
HEAD検証失敗なら本文GET・init/plan・成功receiptへ進まず、GETや再照合で失敗してもその時点でSTOPする。
実サイズを観測済みと記述できるのは、runnerでこれらの検査が実際に成功し確認できた後だけである。

initial stateはHEAD/GET/HEADとpositive lock/workspace LIST absenceで取得する。exact既存11 managed instance、history未登録、既知Terraform state/check envelope、current VersionIdと全bytesをprivateに検証する。STANDARD class・既知暗号化・exact sizeを確認できなければdownload/initへ進まない。403/通信失敗/欠落/truncated responseを不在と見なさない。

init/plan/sanitizer後は`always()`で再snapshot、stable/未知fieldのstrict tables.get比較、volatile観測値のprivate記録、exact native lockのpositive LIST absenceを行う。期限切れや失敗でも保存済みadmissionの上限でpost safety readを試みるが、fresh evidence・全step成功・strict import1/既存11 no-op/他action0だけが成功receiptを作れる。独立post-noop routeも同じ安全入力を使い、import0/no-op12/他0を要求する。

public Summaryは固定PASS/STOP、sanitized action数、waiverとactual cost unknownだけ。raw state、VersionId/serial/lineage、complete metadata、binary planは0600のprivateファイルで扱い、artifact/cacheへ保存せずalways cleanupする。強制停止・権限不足等で完全receiptが得られなければclosureしない。初回history applyではauthentic root emitterを必須にする。具体的な承認v2、source freeze、独立post-noopとSTOP/recoveryは[history packet](docs/development-history-ready-packet.md#one-shot-import-source-contract-not-execution-approval)を参照する。

旧`cost_policy()`と`dev-history-plan-monthly-2026-10-09-v1`は過去のoffline ledger解釈用だけに保持する。live admissionから呼ばない。S3 lock version/delete markerの残存や後月保管費は0・削除済みと見なさない。

## State recovery

backend bucketのObject Versioningを前提に、誤ったstate更新・削除時は以下の順序で停止・復旧します。

1. apply / importを停止し、対象account/environment/backend keyとnative lockを確認する
2. 対象environmentとbackend bucketを再確認する
3. S3 object VersionIdと事故前後のStateをprivateにread-only比較する
4. 復旧候補versionと現行Stateのlineage / serial / ownership / 後続writeを比較する
5. 現行Stateをprivate領域に保全し、候補versionの盲目的な上書きを禁止する
6. native lock下で現行ownershipと後続writeを保持できる、対象限定の復旧planをreviewする
7. 承認された復旧手順だけを実行し、完全な `terraform plan` とversion / lockを再検証する
8. 差分の原因が説明できるまで通常applyを再開しない

dev / prodを跨いだstate copyは行いません。
SSO/MFA、break-glass、責任者、private contactの確認は
[State account recovery runbook](docs/state-account-recovery.md)を参照します。

## Import policy

既存resourceは configuration-driven `import` block を標準とします。

順序:

1. fresh inventory
2. resource definition
3. import block
4. `terraform plan`
5. remote実値を正しく表すまでconfigurationを修正
6. import直後のplanを原則no-opへする
7. importとは別PR / 別差分で設定改善を行う

import acceptance:

- create: 0
- update: 0
- delete: 0
- replacement: 0

bucket / BigQuery / IAM / Scheduler等でdestroyやreplacementが出た場合は停止します。

## Authentication boundary

Phase 1のrepository validationはcloud credentialを使いません。

Issue #1161のdevelopment backend bootstrapはoperatorの短期SSO credentialで完了済みです。長期AWS access key / secret key、Google Service Account JSON keyは作成していません。

通常運用では認証をCIへ寄せます。

- S3 backend: GitHub OIDC → AWS IAM role
- GCP provider: GitHub OIDC / Workload Identity Federation → GCP
- ローカルのauthenticated `plan` / state operationはmigration・障害調査等の例外用途とし、AWS SSO / Google ADC等の短期・更新可能credentialを使う
- public PR CIはcredentiallessを維持する
- development authenticated plan / applyはtrusted ref / GitHub Environment / least privilegeで実装・実測済み
- production backend/trustは実bootstrap済み。正規migration経路は有効で、State初期化・既存30件のownershipと最終no-opは各実行結果で検証する

既存AWS runtime → GCP WIFとはTerraform CI trustを分離します。

## Production boundary

production:

- 最初のimportは同resource typeをdevで確立してから
- 本人初回workflow_dispatch限定・Required reviewersなし。実測Development Exitと独立レビュー済みproduction readiness gateを必須とする
- import直後の想定外diffをapplyしない
- destructive planは停止
- Node.js 22 migrationとTerraform ownership移行を同じ変更にしない


## Bootstrap follow-up

Issue #1161のdevelopment S3 backend bootstrap本体は完了しています。

残件はbootstrapとは分離して追跡します。

- Issue #1165: alternate contacts / recovery運用
- Issue #1166: durable CloudTrail / S3 data events監査
- Issue #1162: public repository向けauthenticated Terraform plan / apply（completed）
- Issue #1191: Development Exit実証済み / Production State・既存ownership移行と最終ReadyGate

#1165の[recovery runbook](docs/state-account-recovery.md)と#1166の
[durable audit source/設定案](docs/state-durable-audit.md)は独立follow-upです。
Audit logging/継続課金は未有効化で、設計案を実装完了と扱いません。

## 完了済みDevelopment waveの履歴

以下の各wave、旧modeとEnvironment承認は当時の記録です。現在は上記の定常migration経路と固定baselineで全所有を保持します。

### Development import wave 1: native backup schedule

`environments/dev/native-backup.tf` は、実環境のdaily backup scheduleをconfiguration-driven importします。これはFirestoreのnative backupであり、#1173で保留しているScheduler / Pub/Sub / export Functionとは別resourceです。

- Same: daily recurrence / `(default)` database
- Intended environment difference: developmentは30日、productionは20日保持。既存値を変更しない
- provider ownership: developmentのschedule 1件のみ。Firestore database本体、backup data、Rules / Indexesは対象外
- Google providerのReadはscheduleのGETだけを使う。dev CI custom roleへの追加は `datastore.backupSchedules.get` だけとし、list / create / update / delete、backup本文read、prod権限は追加しない
- `prevent_destroy` と既存CI import-only guardを維持する。providerのlocal `deletion_policy` defaultはimport時に変更せず、no-opを確認する
- credentialless PR CI PASS後にintegration branchへmergeし、同じSHAのauthenticated plan → Environment-approved import-only apply → post-apply no-opを確認する
- production定義 / import、export chain、API ownership / runtime WIFは別wave

2026-10-05の承認済みoperator waveでdevelopment backup bucketのPAPだけを`enforced`へ修正した。両環境のPAPは一致し、他bucket metadata / IAM bindingは不変。`user-activity-history` のdev-only `timestamp` は不要なlegacy driftだが、値・consumer・再流入・復元確認前のDROPとimportは保留する。

## Development backup bucket import

`environments/dev/backup-bucket.tf` imports the existing retained Firestore export bucket through the shared `retained-backup-bucket` module. Objects, backup data, IAM, production resources and export triggers remain outside this wave. `force_destroy=false` / `prevent_destroy` are required. Region, storage class, retention and soft delete preserve inventory values. Review the module README for the dev/prod metadata contract. Accept only import/read/no-op and require post-apply complete no-op through the existing Environment-approved workflow. A missing CI `storage.buckets.get` permission is an approval boundary, not grounds to add Storage admin.

The obsolete empty development GCS state backend was retired in a separate approved operator wave on 2026-10-05. The old operator's executable config/cache was disabled; empty state and noncurrent init lock generations were conditionally deleted, followed by the bucket. Seven-day soft delete remains enabled. Recovery authority is the current versioned S3 state; never reconnect to the retired GCS backend. Detailed verification belongs to #1161 / #1162 and Notion Current Canon.

## YouTube quota alert symmetry

Both dev/prod roots use `modules/youtube-quota-alerts` with environment parameters. Policy ownership is disabled during import-only migration; the existing production policies are untouched. Read-only preflight found no notification channel in either project, while all three production policies reference a missing channel. Actual dev creation is deferred to a separately approved normal-change apply, after channel verification. Mock plans prove three logical types and unchanged import-only rejection of create=3. See the module README for baseline, future import addressing, and remaining gates.

## Private primary Email preparation

Both environments share `monitoring-notification-channels`; quota policies consume its sensitive channel-name output when ownership is enabled. Development native Email channel was created in a separate approved operator wave; production channel/policies are unchanged. Actual mailbox and channel ID are private Environment secret inputs only, and may enter protected S3 state.

Development Email adoption completed through the dedicated protected route with post-plan no-op5 and unchanged GCP metadata. `DEV_PRIMARY_EMAIL_IMPORT_ENABLED=true` now keeps its ownership in ordinary import-only runs. Real import preflight found a sensitivity-only `labels` update despite unchanged resource values. The global import-only guard correctly rejects that update.

The separately approved `terraform_mode=email-adoption` dispatch uses the same trusted ref, plan/apply Environments, identities and same-SHA re-plan. Its dedicated validator requires exactly the existing development channel import, equal before/after values, no unknowns/drift, only the label sensitivity mark, and the four previously managed resources as no-op. Post-apply must be a complete five-resource no-op with no import. The global import-only sanitizer is unchanged; ordinary `plan`/`apply` still use it. Re-running adoption after success is rejected rather than authorizing a wider update. Rollback authority is the versioned S3 state; do not restore an old version over later state writes. No alert creation or GCP mutation grant is enabled by this route. Delivery/receipt remains untested.

Private recipient and channel name are supplied through the existing two development Environments. Only `storage.buckets.get` and `monitoring.notificationChannels.get` were added to the existing custom read role under #1162 approval; its principals and bindings are unchanged. See the channel module README and #1162 for the historical preflight.

## Development quota normal-change preparation

`terraform_mode=quota-plan` produces a read-only full-root candidate with existing five resources no-op and exactly three fixed quota creates. `terraform_quota_create_gate.py` validates all configured policy fields, the #1175 query hashes, ratios 0.8/0.8/0.6, duration 60s, target project and the adopted Email only. Unknowns are limited to provider-generated policy/condition identities, creation metadata and local deletion policy; unknown notification/config values are rejected. Import/update/delete/replace/drift/other actions are rejected. Public summaries contain addresses/counts only. Post-create requires all eight resources no-op.

Issue #1162 explicitly approved and provisioned one `monitoring.alertPolicies.get` addition to the existing dev read role and one `monitoring.alertPolicies.create` permission in a separate dev apply-only custom role bound only to the existing apply principal. `quota-create` is closed after the successful fixed three-create apply; independent plan/apply Environments, same-SHA re-plan, projection matching and post-plan no-op8 remain mandatory. Runtime identity checks require plan GET only and apply GET + CREATE, and reject UPDATE/DELETE/list/data/IAM/API/production grants. No predefined role or global import-only guard change is used. Ordinary runs keep all eight resources owned after creation; `quota-plan` never executes apply. After successful creation, a separate reviewed follow-up closes the create gate and maintains persistent quota ownership before an ordinary import-only run.

Rollback of a partial create uses a private manifest of the exact newly created policy names and the existing operator's permission: remove only those new policies, preserve the channel/previous five resources, reconcile Terraform ownership under the native lock, then prove no-op5. Do not run broad destroy or overwrite later state with an old S3 version. Terraform `prevent_destroy` and the normal gate intentionally reject deletion; operator rollback is a separate bounded procedure. After successful creation and no-op8, enable persistent quota ownership in a reviewed follow-up before an ordinary import-only run.

## Development quota empty-label state reconciliation

The three-create apply succeeded, but provider read-back represented absent `user_labels` as `{}` while creation saved `null`. All eight normal resource actions were no-op; three representation-only drift records correctly stopped the strict post guard. Explicit `user_labels = {}` prevents recurrence without adding a real label. Post validation also matches the exact observed provider defaults (empty severity/subject/evaluation and zero trigger percent); arbitrary values remain rejected. These defaults are already identical before/after, so they introduce no additional refresh delta. Global import-only policy remains unchanged.

`quota-refresh` completed under Issue #1162 approval with regular post-plan no-op8/drift0; its one-time gate is closed. Its dedicated validator requires a full-root saved `-refresh-only` plan, zero workload resource changes, exactly the three newly owned policy addresses with only `user_labels: null -> {}` (including its corresponding sensitivity mask), eight resources in the refreshed graph, exact approved policy/channel semantics, unknown zero, output no-op and other drift zero. Public summaries retain drift=3 rather than concealing it. Independent plan/apply Environment approval, same-SHA re-plan and projection matching remain mandatory. The saved refresh plan can persist state only; the final regular plan must pass no-op8/drift0. No IAM/API/workload or production write is authorized by this mode.

State versioning retains the previous version for investigation. Do not overwrite newer state to roll back a representation-only refresh; inspect current state under the native lock and require a new bounded plan for any subsequent reconciliation. The correction changed only the three empty label maps. Ordinary full-root plans now have no representation drift.

## Development export chain adoption

Development quota refresh completed with regular no-op8/drift0. PR #1184/#1186 and protected run37342000553 adopted the existing topic; post-plan was import0/no-op9/drift0. Scheduler and Gen1 Function were then adopted in separate protected import-only waves, reaching regular full-root import0/no-op11/drift0. Generated resources, unrelated IAM and source rebuild/upload remain excluded.

## Approved quota state representation correction

Issue #1162 comment5997848683 authorizes the one-time `quota-refresh` route for
exactly the three newly created development quota policies. The separately approved route
does not widen the global import-only policy. The dedicated validator requires a
complete eight-resource full-root refresh-only plan, no workload actions or
unknowns, only `user_labels: null -> {}` and its matching sensitivity map, and
unchanged fixed policy values. Independent plan/apply Environments, same-SHA
re-plan and projection equality remain mandatory. The post-plan is a regular
full-root plan and must be no-op8/drift0. The exceptional gate is closed after success;
never restore an old S3 version over subsequent state writes. No cloud, IAM, API
or production mutation is authorized by this state correction.

## Approved development topic wave

PR #1184 defines the existing topic. Issue #1162 comment5998188927 approves
only `pubsub.topics.get` in the existing development read role and topic import.
The authenticated workflow keeps topic ownership enabled for both identities;
the regular root default remains disabled for credentialless validation. An
additive exact nine-resource validator preserves the global import-only guard:
accept import1/topic no-op plus existing8 no-op, unknown/drift/other0; after
adoption accept import0/no-op9. Independent Environments and same-SHA re-plan
are unchanged. No subscription/IAM/generated/source ownership or cloud config
mutation. Scheduler/Function permissions remain separate approval boundaries.

## Approved development Scheduler wave

Issue #1162 comment6005196430 separately approves only `cloudscheduler.jobs.get`
in the existing development CI read role and one Scheduler import. Existing
principals/bindings, production IAM and APIs are unchanged. The root default
stays false for credentialless validation; protected CI enables persistent
Scheduler ownership only for development, with the adopted topic dependency.

An additive exact ten-resource validator preserves the global import-only
policy: Scheduler import1/no-op plus existing9 no-op, no drift/unknown/mutation
or generated ownership. Both CI identities must have only the approved topic
and Scheduler GETs in the export permissions checked; list/create/update/delete/
run/pause/resume/publish and Function GET remain absent. Resume uses the IAM
permission `cloudscheduler.jobs.enable`, which is explicitly checked as absent.
Independent Environments, same-SHA re-plan, projection equality and saved
import-only apply remain mandatory. Post-plan must be import0/no-op10/drift0.

See [the Scheduler runbook](modules/firestore-export-scheduler/README.md) for
fresh fields, state/metadata audits and rollback boundaries. Function ownership
waits for Scheduler completion; Function GET remains a separate approval.

### Scheduler completion and default-off Function definition — 2026-10-06

Scheduler activation #1188 completed on integration SHA `eb46cb9603f320dbf2a729cafecf8a88ac598c8a`, protected run [37388902196](https://github.com/kani3camp/youtube-study-space/actions/runs/37388902196). Separate plan/apply approvals, same-SHA re-plan and projection equality passed. Pre-plan: Scheduler import1 + existing9 no-op; post-plan and independent regular full-root: import0 / no-op10 / drift0 / other actions0. Only `cloudscheduler.jobs.get` was added to the existing dev read role, with bindings unchanged. Cloud metadata in both environments, IAM/API and generated resources remain unchanged by import; S3 serial12 / resources10 / lineage unchanged / native lock released. Public logs/artifacts leak audit passed.

`environments/dev/export-function.tf` defines the adopted Gen1 Function and remains absent from production. Fresh inventory and protected import confirmed Node22 / ACTIVE / version8, unchanged deployment/source/lock provenance, and the external reserved label/source boundary. The only added Function read permission is `cloudfunctions.functions.get`; source build/upload/redeploy and Google-managed ownership remain outside Terraform. See the [Function module contract](modules/firestore-export-function/README.md).

### Approved development Function activation — 2026-10-06

[#1162 comment6005988785](https://github.com/kani3camp/youtube-study-space/issues/1162#issuecomment-6005988785) approves only `cloudfunctions.functions.get` in the existing development CI read role. Principals/bindings remain unchanged; no list/mutation/call/invoke/sourceCode/IAM/API/broad-role/production additions. The workflow enables the previously reviewed default-off Function definition only for development, after adopted topic and Scheduler dependencies. A harmless exact Function metadata GET supplies the preserved execution identity privately to both jobs.

The unchanged global import-only policy plus `terraform_export_function_gate.py` require Function import1 + existing10 no-op, drift/unknown/unexpected action0, then post import0 / no-op11 / drift0. Independent Environment approvals, same-SHA re-plan/projection equality and saved-plan apply remain mandatory. Runtime Node22/ACTIVE/version8, trigger/retry, environment, execution identity and limits are fixed. Source/redeployment and Google-managed generated resources stay outside ownership; no manual trigger is permitted. See the [Function contract](modules/firestore-export-function/README.md) for provenance and state-only rollback.


### Disabled runtime wave execution receipts

Runtime ownership のfresh-state/apply/per-wave receipt sourceは
[runtime ownership contract](docs/runtime-wif-api-ownership.md)を参照。
既存11/history12のexact guard、same-SHA/private packet/plan seal、one-object state delta、full-root post-noopと
complete unrelated IAM/API/WIF metadata保存を検査する。gateはfalse・issuer catalogは空。
original importは成功しましたが、authentic history root receiptは未取得です。既存v1 receipt/v2 packetのimport/apply履歴契約を維持し、別のcurrent-state attestation/v3 packetを追加します。dummy resultsは実receiptやactivation approvalではありません。
