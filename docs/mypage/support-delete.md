# Support deletion B: workflow と運用 gate

この変更は、確認済みの削除受付を一つの durable execution へ束ねる workflow、Firestore checkpoint / source allowlist adapter、private operator CLI と故障再開試験を提供する。CLI の既定は offline plan。実接続の bootstrap はなく、`--execute` は `LIVE_ADAPTERS_UNAVAILABLE` で拒否する。mock 完了は実データの削除完了を意味しない。実 runtime・Auth・BQ・backup・vendor・log adapter の保証は未確認で、live 完了 gate は開かない。

Stack は MyPage integration → OAuth TTL → SAC A → この B。base は `integration/mypage-canon-20261006`。専用 integration でレビューし、dev/prod の merge・deploy は別工程。従来 Terraform 移行の順序と、[raw archive retirement](../privacy/raw-live-chat-archive-retirement-runbook.md) / [mixed snapshot cleanup](../privacy/gcs-raw-chat-cleanup.md) の対象・整合性条件を変更しない。

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

[inventory](support-delete-inventory.md) と [machine-readable catalog](support-delete-inventory.json) は source の事実と live unknown を分ける。Manifest.Ref は trusted registry が target environment/project/database、全 writer/launch 経路、開始前状態、旧 queue/import/restore の除外方針、schema 帰属、workflow plan revisionを確認して発行する digest。operator JSON の文字列だけで発行してはいけない。稼働中のcheckpoint cursorを別planへ読み替えない。plan変更時は新manifestの照合で停止し、既存executionの明示的migrationを別途検証する。

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

live は別途明示された運用として、development → production の順で開始する。まず source inventory を実 deployment/old revision/direct launch/export/restore と照合し、全 runtime adapter の pause/drain/resume と ownership、全 store adapter の全件消去・readback・旧 source 再投入禁止を個別に検証する。無保証の scope を成功扱いにする option は追加しない。実 SDK credential の取得、pause、revoke、削除はこの PR の検証に含まない。

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
