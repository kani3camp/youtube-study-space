# Support deletion B: workflow と運用 gate

この変更は、確認済みの削除受付を一つの durable execution へ束ねる workflow、Firestore checkpoint / source allowlist adapter、private operator CLI と故障再開試験を提供する。CLI の既定は offline plan。実接続の bootstrap はなく、`--execute` は `LIVE_ADAPTERS_UNAVAILABLE` で拒否する。mock 完了は実データの削除完了を意味しない。実 runtime・Auth・BQ・backup・vendor・log adapter の保証は未確認で、live 完了 gate は開かない。

[raw archive retirement](../privacy/raw-live-chat-archive-retirement-runbook.md) / [mixed snapshot cleanup](../privacy/gcs-raw-chat-cleanup.md) の対象・整合性条件を変更しない。

## 実行契約

入力は削除の操作意図であり、proof、停止、drain、store 不存在を証明しない。初回は `support-requests` の verified delete receipt、consumed support OAuth の purpose/environment/request binding、request index、target/channel/proof を同じ transaction で照合する。元の acceptedAt と acceptedAt + 7 calendar days の deleteBy を保持し、reissue や再開で SLA をリセットしない。7日超過後の修復は期限内成功と扱わず、終端 timestamp から超過を監査する。

fresh verification は既存の challenge 24h / OAuth 10m 契約による request-bound な対話認証。execution 開始に新しい任意の最大経過時間を追加しない。初回に OAuth trace が TTL で失われている場合は unknown として拒否する。検証を通った proof claim、privacy ON、execution 作成は原子的。同じ execution は既に binding された proof を使って再開でき、途中で OAuth trace が消えても再認証を強要しない。別 execution / target / proof / manifest へ流用できない。

`support-delete-executions/{executionRef}` と `support-delete-proof-claims/{proofRef}` は server-only。checkpoint は selector、owner、revision、control generation、guardSince/cutoff、SLA、固定 inventory manifest digest、cursor、evidence chain digest を持つ。TTL・lease timeout・自動 takeover はない。worker crash 後の owner 変更には、同じ execution/revision/generation の前 worker 停止を独立した runtime adapter が確認した新しい opaque trace が必要。待ち時間や handler count 0 は停止・SDK commit・mint/配達 drain の証拠にならない。

全 29 effect step の順序は固定で、入力から scope を省略できない。

1. 全 mapped Auth identity の revoke。
2. MyPage と legacy fleet の pause、両 fleet の drain。
3. 専用 seat removal、primary、web、OAuth、support relations、legacy mapping、BQ、backup/export、derived/vendor、platform log の消去。Canon 05に従い、Auth userはこれらの消去後に削除する。
4. 全 11 scope の不存在と旧データ再生成禁止を、pause 中に再確認。
5. manifest に記録された開始前の有効状態へ両 fleet を戻す。古い queue、callback、mint result、snapshot、import、restore の再生を許さない。
6. current requestRef と最新 control generation の CAS で privacy だけを解除し、receipt / execution を最小の終端 audit にする。

各 effect は selector、manifest、generation、cutoff、step、stable operation ID、fresh observation、opaque trace を照合する。unknown、欠落、別 scope、pending/remaining > 0、fleet 全体の停止未確認、restore 除外未確認は失敗となり cursor を進めない。adapter は同じ operation ID の timeout / acknowledgement loss を postcondition で reconciliation する。checkpoint revision/owner はすべての commit を fence する。

独立 moderation の generation が変われば同じ privacy request/guardSince を確認して最新 generation を採用する。途中の absence verification はやり直す。一度 durable resume intent に進んだら削除・inspection 段階へ戻らない。resume 後に Bot が作成した正当な新規データを、finalize 障害時の再試行で消してはいけない。privacy clear は独立 active moderation を維持する。全 inactive SAC checkpoint も generation とともに保持し、TTL、削除、reset を行わない。

終端 receipt は schemaVersion、acceptedAt、deleteBy、completedAt のみ保持する。execution terminal には同じ selector + manifest の完了照合専用 digest を追加し、別 target/request/proof へ completion を転写できないようにする。raw link は保持しないが、この opaque digest を匿名化の完全な証明とは扱わない。channel、proof、owner、request ID、OAuth/index link、manifest、evidence chain を除去し、current OAuth、request index、proof claim を transaction 内で消す。opaque completion ID の再照会は effect を実行せず完了を返す。新しい保持期間は導入しない。外部に保管した operator manifest・ログ・export 自体の消去は inventory scope の責務であり、これだけで匿名化済みとは主張しない。

## adapter の実装境界

[削除対象と起動経路](support-delete-inventory.md)で、ソースから確認できる範囲と実環境で確認すべき範囲を区別する。Manifest.Ref は trusted registry が target environment/project/database、全 writer/launch 経路、開始前状態、旧 queue/import/restore の除外方針、schema 帰属、workflow plan revisionを確認して発行する digest。operator JSON の文字列だけで発行してはいけない。稼働中のcheckpoint cursorを別planへ読み替えない。plan変更時は新manifestの照合で停止し、既存executionの明示的migrationを別途検証する。

`FirestoreDeletionStore` は injected client の project/default database path を検証する。`FirestoreDeletionEffects` は以下の source allowlist を実装し、trusted RestoreGuard が同じ operation に pause/drain・catalog completeness・restore 除外を証明するまで書き込まない。

| scope | implemented source allowlist / limitations |
| --- | --- |
| seat-state | seats/member-seats の user-id query を専用消去。通常 Out/Block の activity/RP/Ban を生成しない |
| firestore-primary | users direct doc、work segments、activity、order、4 seat limits、legacy history の field query |
| firestore-web | channel WebAccount doc。process cache/flight/配達の確認は runtime adapter 必須 |
| firestore-oauth | channel OAuth と current receipt の support OAuth query。current verification trace は finalize まで保持。channel 未確定 callback は runtime drain 必須 |
| firestore-support-relations | current challenge / duplicate request index の消去。canonical index / current claim は finalize まで保持。他の target receipt / owned execution / proof claim があれば unknown で停止し、勝手に completed にしない |

削除は先頭の bounded page を transaction ごとに取り直し、同じ owner/revision/generation を読んでから delete する。partial commit をまたいだ offset による取りこぼしを避け、空 page を再確認する。101件 × 10,000 page が上限で、上限到達は incomplete。最終 inspection で非空なら失敗。別 channel・SAC・current execution/proof を消去しない。

Registry の未注入 scope は常に unavailable。legacy mapping の逆/重複 UID と古い proof/receipt 関係、Auth tenant の全 identity と in-flight mint、BQ の main/tmp/job、mixed/soft-deleted backup・PITR、channel index のない trend/vendor/log は、API の保証と帰属が確定するまで live adapter を作らない。ソースの `GetAll` や query 0件を全store不存在へ拡張しない。既存 runbook の対象外である混合 backup の一括削除をこの CLI で代行しない。

## 合成 adapter の契約

全 adapter は合成検証用。実環境への接続・権限・完全な対象 inventory・削除結果の readback が揃うまで live は拒否する。

### Auth と関連レコード

`FirebaseDeletionEffects` は injected fake SDK と Firestore Emulator で試験する adapter。既存 `Operation` / `Evidence` / stable operation ID / 29 step を変更せず、`firebase-auth`、`legacy-mappings`、`firestore-oauth`、`firestore-support-relations` を一つの ownership snapshot に基づいて扱う。Registry / CLI / server の live bootstrap には接続していない。`live` manifest は拒否し、`--execute` の `LIVE_ADAPTERS_UNAVAILABLE` と全 live gate を維持する。

最初の revoke より前に、独立した `AuthOwnershipInventory` から full execution binding、complete な UID / tenant / user 作成時刻 / exclusive channel owner、legacy mapping と関連 document 全体の fingerprint を受け取る。空の UID list も明示的な complete inventory が必要。channel 文字列を現在の UID と推測したり、tenant を既定値にしたり、`firebase-uid` field 一つを ownership としたりしない。Emulator は direct legacy UID document、reverse mapping query、receipt / OAuth / challenge / request index と逆参照を照合し、欠落・共有・別 channel・上限超過を拒否する。fixture inventory の注入は実環境の完全性を証明しない。

snapshot は server-only `support-delete-auth-ownership/{executionRef}` に保存する。selector、manifest、owner、revision、generation、cutoff の binding は checkpoint の Commit / Refresh / Recover と同じ transaction で持ち越す。legacy mapping の削除と `mappingsRemoved` は原子的。再開時は mapping の不存在から旧 UID を推測せず、同じ snapshot を使用する。後続 step の snapshot 欠落や差し替えは停止条件。終端 Finalize は receipt / execution の最小化、current OAuth / index / proof claim の消去、snapshot の消去、privacy clear を同じ transaction で行う。snapshot に独立した恒久 retention / TTL / SLA を導入しない。

初回 Acquire は有効な OAuth trace を検証し、proof claim と execution を原子的に保存する。その後の同一 execution は、current receipt / proof claim / privacy fence の binding を照合し、snapshot 保存前後の current OAuth の TTL 消失を許容する。TTL 消失済みの trace は complete inventory に残っていても省かれていてもよいが、残存・再作成された OAuth は inventory と fingerprint を必須とし、shared OAuth の逆参照も検査する。missing / 別 request / 別 execution の claim は拒否する。再認証や SLA のリセットは行わない。

SDK の `GetUser` / `RevokeRefreshTokens` / `DeleteUser` interface は pinned Firebase Admin SDK の project client と tenant client が満たす。constructor、ADC、認証、実 API 呼出しは追加していない。project / tenant client と NotFound classifier は明示注入し、permission、timeout、unknown は不存在へ変換しない。SDK の success response も完了の証拠とせず、同じ UID / tenant / user 作成時刻の definitive readback で revoke または不存在を確認する。lost acknowledgement 後も同じ operation ID と snapshot で結果を照合し、UID 再利用を検出したら新しい user を消さない。

`DeleteUser(uid)` は incarnation 条件付き CAS ではない。`AuthDeletionGuard.WithGuard` は SDK の read / mutation / readback を含む callback 全体で UID 再作成・import・所有変更を排除する独立保証を必須とする。snapshot の作成時刻だけではこの保証を満たせない。guard は同じ operation の trusted fleet / restore evidence、同期 clock、全 token consumer の check-revoked-token gate も明示する。これらが欠ければ mutation を始めない。revoke の `TokensValidAfterMillis` は UTC 秒粒度なので cutoff の次の秒以上を確認する。revoke acknowledgement 自体は既発行 ID token の失効を証明しない。既存の全 token consumer、mint / delivery、再投入排除の実保証は外部 gate のまま。

関連削除は独立 inventory に含まれ、他の owned execution / proof claim を持たない receipt とその OAuth / challenge / request index に限定する。別の active execution、shared proof / OAuth、unknown relation があれば transaction 全体を止め、他の receipt を勝手に completed にしない。current receipt、current verification OAuth、canonical request index、current proof claim は Finalize まで保持する。別 channel、SAC、他 execution は削除しない。

fake SDK tests は wrong UID / tenant / incarnation、shared owner、missing inventory、guard 欠落、秒境界、timeout / lost acknowledgement / retry、UID 再利用を確認する。Emulator tests は durable snapshot と mapping 削除後の再開、reverse owner / inventory 欠落の rollback、関連閉路と他 channel isolation、owner recovery / generation fence、atomic terminal minimization、anonymous / authenticated Client の read / write deny を確認する。これらは実 UID / tenant / IAM、fleet、BQ、backup、vendor の不存在や全 live deletion 完了を証明しない。

### BigQuery

`BigQueryDeletionEffects` implements the existing `bigquery` scope using only an injected `SyntheticBigQueryExecutor`. No SDK executor, credential discovery, real query, IAM change, runtime integration or CLI bootstrap is added. `live` is refused, and `--execute` still returns `LIVE_ADAPTERS_UNAVAILABLE`. Synthetic completion is not live deletion evidence.

An independent complete inventory pins the execution/manifest/cutoff, explicit project/location, required activity/order/historical-raw main tables, shared `tmp`, inventoried result tables, old load/query/copy/export job identities and opaque external-copy references. Existing tables require a verified schema and incarnation fingerprint; an empty or missing schema is unknown, not zero. Explicitly absent retired tables are reobserved through the catalog and target-count port. Identifiers use a restricted allowlist; channel values are bound as `@channel`, never interpolated into SQL. The pure planner constructs SQL only and cannot run it. Dataset/table-wide deletion and the raw-retirement purge are not reused.

The catalog port must enumerate the full frozen source/job/copy set, accounting separately for this inventory's deterministically planned deletion jobs. Extra, missing, changed or incomplete metadata blocks execution. Old source jobs must have definitive terminal status and replay exclusion before mutation; caller/task cancellation and cancel acknowledgements are insufficient. The independently trusted guard holds catalog/incarnation exclusion during each callback and attests the paused fleet, job fencing and exclusion of old imports/restores through the destructive window. A fixture's `true` flags do not prove a real fleet is stopped.

Server-only `support-delete-bigquery-inventory/{executionRef}` stores the immutable inventory, deterministic job-plan inputs and deletion/inspection confirmations. `support-delete-bigquery-bindings/{executionRef}` and an optional immutable `bigQueryInventoryRef` capture marker in the active execution document are created atomically with it. Loss or replacement of either temporary document, including loss of both, cannot trigger recapture after partial effects while the execution marker survives. Older executions without this marker remain readable; ordinary checkpoint updates preserve it and terminal replacement removes it. Restore of the whole pre-capture execution remains forbidden by the independent restore guard. A job ID/configuration digest is stable across retry, owner recovery and independent moderation changes. Submission success or timeout must be reconciled by a definitive lookup of that exact project/location/job/configuration. A pending, failed, missing-at-inspection, mismatched or unknown job stops the workflow. All table target counts must be definitive zero and freshly observed; the catalog is rechecked after processing.

The snapshot execution fence moves in the same Firestore transaction as Auth ownership and workflow checkpoints. Every table operation rechecks it together with the same durable proof claim, receipt and canonical request index; OAuth TTL disappearance does not require renewed proof. A late effect cannot publish valid evidence after owner/generation drift. This transaction cannot make an external operation atomic, so the independent guard and durable remote-job identity remain required. Generation changes before durable resume intent invalidate inspection confirmation. Finalization atomically removes both BQ documents with Auth snapshot/receipt link cleanup and privacy clear; inactive SAC generation and independent moderation remain. No new permanent retention/TTL is introduced.

External GCS/vendor/log copies are not deleted by this adapter. They are sealed against replay before BQ deletion, processed by their owning scopes, and their request-bound disposition must be attested before BQ inspection. Unknown copies block completion; main-table zero never proves whole-store absence. Live time-travel/backup/export/log residuals, actual API guarantees, full job/table inventories and permissions remain engineering/operational gates. This synthetic executor does not establish those facts or justify a new retention policy.

Stateful synthetic tests cover target/nontarget rows in main/tmp/result tables, schema/incarnation/catalog drift, wrong project/location, old pending/unknown jobs, lost submission/checkpoint acknowledgements, stable recovery identity, stale/failed counts, external-copy uncertainty and missing durable state. Firestore Emulator tests cover atomic snapshot movement with Auth, generation/recovery fences, half-state loss, rollback/terminal cleanup, client denial, engine refusal before Auth/resume/finalize, and finalize retry preserving legitimate post-resume data. Other required engine scopes remain explicit fixtures or unavailable, never implicitly successful.

Use the existing repository checks: Go tests, targeted race, lint, root Firestore Emulator script, MyPage contract and documentation-reference checks. They do not validate real BigQuery SQL execution or live absence.

### 派生データとvendor

`DerivedDeletionEffects` implements the existing `derived-vendor` step with injected inventory, guarded persistence, and a stateful synthetic executor. It makes no vendor call. `live` remains unavailable and the CLI `--execute` remains `LIVE_ADAPTERS_UNAVAILABLE`; a fake's deletion/readback cannot prove real OpenAI, Discord, YouTube, or cache disposal.

The inventory explicitly enumerates BFF cache, global work-name trend, OpenAI stored Responses, Discord messages, and service-created YouTube copies. Independent moderation/YouTube Ban state is not a privacy deletion target. Every source must attest completeness and disposal support, even if it reports zero objects. An item has an exact object ID, incarnation fingerprint, complete owner-channel set and supported disposition. Exclusive items may be deleted as objects; shared items require target-scoped removal and a readback that preserves every other owner. Missing attribution, unknown catalog, unsupported disposition, changed incarnation, unexpected target copy, and uncertain readback stop the step. A global ranking without source ownership cannot be inferred to be target-free merely because its stored schema has no channel field.

The source inspection found `Store: true` on OpenAI Responses but no persisted response ID, and `ChannelMessageSend` discards the returned Discord message ID. The legacy trend output contains shared examples without channel attribution. Therefore the live inventory and deletion capability remain unknown. This package does not add a guessed ID field or a retention period. The synthetic fixture's object IDs exist only in the test fake. YouTube copy ownership remains unknown; platform logs have their own separate scope.

The server-only snapshot and binding are captured atomically with an immutable marker on the active execution. Owner, revision, generation and cursor move with the checkpoint; losing either snapshot half after capture cannot cause recapture. Mutation IDs derive from the captured operation, inventory, source, object and incarnation and survive recovery. A timeout/lost acknowledgement is followed by a definitive read. Partial completion retries the same IDs and observes already disposed items. `inspect` rechecks every item and the full catalog before advancing. Generation changes before durable resume intent invalidate the inspection confirmation. Finalization removes the temporary snapshot and marker while independently active moderation remains.

The Firestore Store also refuses a checkpoint that advances the derived delete step without this snapshot, even if a caller injects otherwise valid generic Evidence. Once that step has advanced, absence blocks inspection, recovery and finalization. Steps before derived capture and explicit recovery at the uncaptured delete cursor remain possible. A missing or mismatched snapshot/binding/marker half always blocks recapture after capture.

Before any live wiring, an operator must prove source-time channel ownership, actual IDs and provider-supported target disposal, old writer/queue exclusion, exact readback semantics, other-owner preservation and restore exclusion. A mixed shared copy with no safe target-scoped operation stays blocked; it must not be destroyed wholesale. D02's design choice is: do not store refresh tokens or add periodic polling solely to detect external Google grant cancellation. Its policy compliance and handling of retained API data after cancellation still need verification/compliance review. D03 public operator/contact/URL values remain undecided. This synthetic adapter does not authorize live deletion.

## Synthetic Registry と開始前 assembly

[Registry](../../system/core/supportdelete/registry.go)はtrusted fixtureから渡された接続を開始前に検査する。runtime 2件とdeletion 11件、既存29 stepの必要actionがすべてあり、scope重複・未知action・欠落・nil/typed-nil adapter・target/mode不一致がない場合だけ、privateな接続mapへコピーする。呼出し元の登録slice/actionを書き換えても選択済みadapterは変わらない。adapter内部のmutable stateを複製・静止する機能ではなく、各adapter自身のguard/fenceを必要とする。

[PrepareDeletion](../../system/core/supportdelete/prepare.go)は独立した合成catalogとselector/manifestを照合し、Registry検査後だけ明示注入されたStore factoryを呼ぶ。不足時はfactory/Acquire/recovery/effectを実行しない。prepare成功自体もproof claimや削除ではない。manifestのRefはtrusted fixtureが供給し、文書やoperator入力からlive証跡を発行しない。`development`の`demo-` projectと`mock`/`emulator`に限り、`live`は完全なfake接続でも拒否する。SDK constructor、ADC discovery、実endpoint選択は追加しない。注入factoryのclient lifetimeはcallerが管理する。

戻るPreparedDeletionはselector/manifest/接続を公開変更可能にせず、Runと明示Recoverを同じbound executionへ限定する。同一assemblyの同時呼出しはbusyで拒否する。このprocess内lockはfleet停止・drainの証拠ではない。Recoverのproposalだけではownerを移さず、Storeがstopped traceとdurable revision/generationを検査した後だけownerを更新する。Recoverのtimeout/ACK喪失は未確認として返し、ownerが移ったと推測しない。明示した新ownerの同一bindingで別assemblyを作り、authoritativeなAcquireの結果で再開可否を確認する。

RunMockの通常fixtureとstopped-worker fixtureは両方この経路で組み立てる。既存29 step、stable operation ID、acceptedAtからの7日、proof/generation/checkpoint、再開後の正当な新規data保護はEngine/Storeの既存契約を維持する。CLI schema/flags/reportは変更せず、`--execute`はSDK/bootstrapやmock runnerへ到達せず従来の`LIVE_ADAPTERS_UNAVAILABLE`で拒否する。live catalog発行、operator削除権限、全fleetと各実storeのadapterは未接続のまま。

## offline CLI

`MYPAGE_ENVIRONMENT` と `GOOGLE_CLOUD_PROJECT` を明示する。alias project env に不一致がある場合や live credential/endpoint env がある場合は拒否する。対象を stdin または exact 0600 regular file の `--manifest` で渡す。最終 symlink/FIFO、16 KiB 超過、duplicate / case-variant / unknown / missing JSON field、確認の不一致を拒否する。report に target、ref、path、raw error は出力しない。

合成 fixture 専用 manifest の形:

```json
{
  "schemaVersion": 1,
  "operation": "delete",
  "target": {"environment":"development","projectID":"demo-youtube-study-space-ci","channelID":"UCsynthetic0000000000001"},
  "requestRef":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "executionRef":"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "proofRef":"2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "confirmation": {
    "operation":"delete",
    "environment":"development","projectID":"demo-youtube-study-space-ci","channelID":"UCsynthetic0000000000001",
    "requestRef":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "executionRef":"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "proofRef":"2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}
```

```sh
cd system
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=demo-youtube-study-space-ci \
  go run ./cmd/mypage-support-delete --manifest /tmp/private-synthetic-case.json
# --check-config: offline input/config check, no Store/SDK/ADC access
# --mock: fixed synthetic case only; --mock --recover: stopped-worker fixture
# --execute: always LIVE_ADAPTERS_UNAVAILABLE, no live SDK bootstrap
```

`MOCK_COMPLETED` / `mock-only` / `offline:true` / `actualExecution:false` を実 deletion の証跡へ転用しない。mock の refs を変えても fresh proof は生成されない。

## live 前の engineering gate と判断

live は別途明示された運用として、development → production の順で開始する。まず source inventory を実 deployment/old revision/direct launch/export/restore と照合し、全 runtime adapter の pause/drain/resume と ownership、全 store adapter の全件消去・readback・旧 source 再投入禁止を個別に検証する。無保証の scope を成功扱いにする option は追加しない。実 SDK credential の取得、pause、revoke、削除は合成検証に含まない。

live adapter の未実装、API の停止保証、コピー帰属、missing proof trace は engineering / external verification gate。これらを「ユーザー承認で成功とする」判断待ちに変えない。inactive checkpoint 保持、fail closed、cutoff、同一 execution 再開、requestRef + generation CAS は承認済みの技術判断。

追加の人間判断が必要になるのは、実 inventory で既存 Canon の7日以内 accessible copy 消去を満たせず例外・期限変更が必要な場合、対象者の帰属を証明できない共有 backup/vendor copy の破壊が他者データの扱いを変える場合、または新しい保持期間/利用制限を提案する場合。現在はそうした変更を実装しておらず、live は fail closed のまま。

## 検証

pure workflow は全29 step の before/after effect failure、checkpoint acknowledgement loss、同じ ID での retry、全必須 scope と binding、owner recovery、generation と新規利用の境界を deterministic fixture で試験する。Firestore Emulator は実 source adapter の proof/guard/claim atomicity、concurrent first claim、missing trace rollback、SLA維持、owner recovery、generation/request CAS、moderation 保持、link scrub、1002件 pagination/他channel isolation、client rules deny を確認する。これは live Auth/BQ/backups/vendor 不存在を証明しない。

```sh
cd system
go test -shuffle=on ./...
go test -race -shuffle=on ./core/supportdelete ./core/mypage ./cmd/mypage-support-delete
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
# repository root
bash .github/scripts/run-firestore-integration-tests.sh
python3 .github/scripts/check-doc-references.py
```
