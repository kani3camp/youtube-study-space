# 開発履歴Tableの観測属性を同期する別契約

CI #885の通常planはimport0 / no-op12 / drift1で停止した。対象は履歴Tableの`etag`と
`last_modified_time`だけだった。State/HEAD不変、安定Table設定不変、lock/workspace不存在、freshnessの
5検査はPASSしたが、通常full-root no-opはSTOPで、issuerはない。この結果を成功へ読み替えない。

[旧prototype](history-observation-sync-prototype.md)は変更しない。M20で追加した専用mode
`dev-history-observation-sync`は導入時に既定無効とした。実装の存在は実行許可や移行完了を表さない。
既存55guard、通常drift0、State不変検査、既存issuer/catalog、HCL、productionを変更しない。
本人の追加操作や実State変更は、このsource変更では発生しない。

## 最新の専用source window（2026-10-16）

専用2gateだけをtrueにし、UTC `2026-10-16T04:29:31Z` から `2026-10-17T04:29:31Z` までの最大24hに固定する。
最終実行SHA Rはこのsourceの統合後に確定し、同Rでpreviewを先に行う。実previewの独立QA後だけ、
同R・同request・期待値B固定の別synchronize runを作り、TableのETAG/LAST_MODIFIED_TIMEと検査結果内容不変
（配列順は変化可）に限定した新しいapply Environment本人承認を必要とする。
source windowはactual admissionやState書込許可ではない。この変更時点でpreview/applyは未実行、
State同期・新issuer・migration Readyは未成立で、本人への実run承認依頼もまだ行わない。
旧4runtime gate、metadata/authority/current-state read、production、profile契約、既存binding/nonceは変更しない。
期限切れ入力や旧run承認を流用せず、このwindowを後から延長しない。

## 対象と承認

対象はsourceで固定したdevの既存12管理対象で、通常resource_changesが全no-op、唯一のresource_driftが
履歴Tableの上記2属性だけであることを要求する。欠落・未知・重複・move・import・replacement・unknown・
sensitivity変更・余分なdrift/actionを拒否する。時刻は非bool整数で増加、etagは非空stringかつ変化が必要。

M20導入時のsource gate `DEV_HISTORY_OBSERVATION_SYNC_ENABLED` / `_APPLY_ENABLED`はfalse、時刻は空だった。
有効化する場合は別reviewでsame SHAに両gateと最大24hのwindowを固定し、PLANを先に行う。
source gateが開いているだけでState変更を許可しない。通常applyのgateはfalseのままである。

公開input `terraform_history_observation_sync_profile`は新kindのrequestのみを受理する。
同じexact SHA、request_id、発行・失効UTC、固定4MiB承認上限、fresh確認したState writer静止を持ち、
operationは`preview`又は`synchronize`。後者には認証可能なpreview_referenceが必須である。
24hは本人の待ち時間を許容するrequest期限で、metadataを24h古く許す意味ではない。
公開request_idは既存private binding nonceとは別物で、nonceを新規発行・置換しない。
別modeへの新profile混入、新modeへの旧admission混入は認証前STOPする。
既存Secretはlegacy又は既存current-state型のexact binding envelopeだけを読み、fullpacketや混在を拒否する。

1. 新しいpreview runの既存`terraform-dev-plan` Environmentを本人が承認する。
2. same-SHA OIDC / 正規plan identityでfresh State全HEAD/byte、metadata、通常planとrefresh-only候補を読む。
3. 独立QAで専用previewを確認した後、同SHA・同requestの**別apply run**を作る。
4. `terraform-dev-apply` Environmentの新しい本人承認を、履歴Tableの2属性と検査結果内容不変
   （検査配列の並べ順は変わりうる）に限定した新たなState同期許可とする。旧runの承認を流用しない。
5. 承認後に全観測・候補を作り直し、previewと同じ期待値Bであることを照合する。BがCならSTOPする。

## 全Stateと保存planの照合

[専用pure validator](../scripts/validate_history_observation_sync.py)はraw drift1を先に厳密検査する。
既存55guardへはresource_driftだけを空にしたdeepcopyを**validation-only**で渡し、そのcopyを捨てる。
summary、issuer、Terraform、State書込へcopyを渡す経路はない。専用結果はdrift1を保持し、実行許可を返さない。

[archive verifier](../scripts/history_observation_sync_archive.py)はbounded ZIPの全entry/CRCを検査する。
`tfstate-prev`の公式header stub（lineage空/serial0）以外の全payloadを元Stateへ照合し、schema upgrade等を拒否する。
`tfstate`の全resource/instance/provider-private/dependencies/identity/outputsをraw Stateと型込みでjoinし、
2属性だけの変化を検査する。show JSONのschema、identity、dependencies、raw sensitivityとschema sensitivityも照合する。
Stateのserial増加候補はexact +1、lineageと他の全JSON内容は不変で、resource/instanceの並べ替えも許さない。

[check-set verifier](../scripts/history_observation_sync_checks.py)は公式Core v1.16.4 protobuf Plan.Checks(field19)、
show JSON checks、元State checksをsource catalogに対して全件joinする。config/object addressはtypedかつunique、
全status PASS・failure message空、未知field/enum/wireを拒否する。検査結果のconfig/object配列の順序だけを
canonicalizeし、null/空配列、optional field有無、他配列や内容は勝手に正規化しない。
CoreがPlan.Checksを適用時に復元することを反映した比較用objectはメモリだけに置き、Stateやbinaryへ書き戻さない。

private作業copyはsame-SHA tracked closureのTF設定とschema/template依存だけをgit blobと完全一致させる。
TFがarchiveへ保存する`.tf`群と、`file()` / `templatefile()`の補助依存は分け、いずれもCLI直前に全byteを再照合する。
`.terraform`、untracked、auto tfvarsをsourceへ混ぜず、未対応`.tf.json`追加はSTOPする。

## 専用previewと適用の制御

[controller](../../../.github/scripts/terraform_history_observation_sync.py)は既存reusable workflow内の別jobだけで動く。
preview/applyは自分のEnvironment、same-SHA OIDC、既存AWS RoleId/accountとGCP SAに限定する。

previewの公開artifactは新kindのJSON1件、8KiB/ZIP32KiB上限、retention1日。
固定counts/field codes、公開run/source/request情報、既存nonceによるpurpose別HMACだけを載せる。
raw State/metadata/plan/属性値/nonce/裸のprivate hashは公開しない。既存issuer型には受理されない。
GitHub exact run/attempt/source/job/本人Env承認/required steps・cleanup/sole artifact/TTLをGETで認証する。

共通intentは安定Table設定全体＋ETAG/LAST_MODIFIED_TIMEの期待値B、元State HEAD/byteと全期待State、
source/private inputsへHMACで拘束する。REST側の既存明示volatile metricsは共通intentへ加えないが、
Stateの同名属性や未知REST fieldを除外しない。State writer静止から業務BigQuery writer停止は推定しない。
新run ID/観測時刻/保存plan binary digestはcross-run共通intentに入れず、provenance又は同job sealに分離する。

適用直前も全HEAD/byte、metadata B、function identity、lock/workspace、source/auxiliary/input/backend cache/
binary sealと期限を再確認する。最後のreadを短い60秒以内に限定し、fsync後も再確認する。
CLIの専用attempt recordを0600/exclusiveで予約し、同じ作業領域の別processでも二重送信を拒否する。
保存refresh-only planのCLI call attemptは最大1。timeout/unknownで再送・rollback・unlockをしない。
予約数とcall attempt数は別に表示し、native内部retry/PUT回数や実費はUNKNOWNを保つ。

適用後は新VersionId、serial +1と全State意味差分、metadata B、lock/workspaceを確認し、別の通常planで
import0/no-op12/drift0を必須にする。それでもこのmodeからissuerやmigration_readyを発行しない。
後段の別通常current-state baselineと独立QAが必要で、継続writer下で成功を保証しない。

全Terraform stdout/stderr/TF_LOG・障害時`errored.tfstate`/backup/provider logs/config snapshotは
owned0700の専用cwdへ閉じ、cleanupで全削除する。失敗時の安全readはbounded best-effortで、期限切れ等はUNKNOWNである。
公開するのは固定stage・予約/call件数・検査PASS/NOT_OBSERVEDだけで、例外や値を出さない。

## 証拠と残る限界

完全隔離network-noneの公開ダミー1Table＋History module/outputs/checks witnessで、公式TF1.16.4/Google8.5の
ordinary drift1→refresh-only saved plan→saved apply→post drift0を確認した。module witnessの元raw-list比較は
check_results順序差でSTRICT_DIFF_STOPのままで、後のtyped-map意味検査だけが内容不変を確認した。
これを実dev full12のnative証明、実行許可、実State観測、migration Readyへ昇格しない。
full12はsource-owned graph＋既存55guard＋synthetic全要素/negativeで検査し、実runで初めてfresh観測を確認する。

独立の直前HEAD/byte検査はTerraform native lockの**前**である。Core lock取得後のsaved-plan staleness照合は
lineage/serialで、全byte/VersionId照合ではない。同serialを書き換える非協調writerのraceを完全排除したとは主張しない。
既知State writers静止、直前全byte照合、native lock/stale-plan、post全差分を重ねるが、この限界は残る。
manual State push、lock=false、独自互換lock、modified binaryで穴埋めしない。

公式根拠:

- [TF1.16.4 planfile reader](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/plans/planfile/reader.go)
- [config snapshot](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/plans/planfile/config_snapshot.go)
- [Plan protobuf・unordered checks](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/plans/planproto/planfile.proto)
- [State v4 serialization](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/states/statefile/version4.go)
- [Core apply Checks復元](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/terraform/context_apply.go)
- [local backend lock/stale-plan](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/backend/local/backend_local.go)
- [保存失敗時のEmergencyDumpState](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/backend/local/backend_apply.go)
