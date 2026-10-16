# HistoryのState観測値を限定同期する案（実行不可）

この文書とpure validatorは、追加の許可を判断できるよう差分を具体化した試作品である。
運用workflowへ接続せず、CLI・認証・Cloud / State / Secretの読取や変更を実装しない。
成功結果でも`execution_permitted=false`、refresh-only候補は常に未証明としてSTOPする。
既存のstrict `drift0`判定と全gateを保持する。現時点でState変更の許可はなく、本人の追加操作は不要である。

## 実測した停止理由

本人承認後の[CI #885 / run 38836240227](https://github.com/kani3camp/youtube-study-space/actions/runs/38836240227)は、
source `8616c732cb57f34c48e8fc2eccb40a9afba97653`、plan job `116765947027`、attempt1でSTOPした。
通常planのresource changesはno-op12、他action0、resource driftは1件だった。
固定診断は`USER_ACTIVITY_HISTORY / update / BOTH`で、VALUE分類の`ETAG` / `LAST_MODIFIED_TIME`だけを示した。
presence / null / unknown / sensitivity / replaceの5分類はNONEである。
State本文・HEAD不変、安定table metadata不変、lock不存在、workspace不存在、fresh policyの5 checkはPASS、
full-root no-op / job outcomesはSTOP。post-auth / cleanupは成功、applyはskipped、artifact0だった。

公開provider schemaではこの2属性はcomputedで、providerのReadが取得値を設定する。
根拠は[Google provider v8.5.0のTable schema / Read](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/bigquery/resource_bigquery_table.go)と、
[Terraformのrefresh-only planning mode仕様](https://developer.hashicorp.com/terraform/cli/commands/plan#planning-modes)である。
computed属性という性質は、実Stateを書き換える許可や既存drift判定の例外にならない。
今の許可済みmodeはこのState変更を扱わず、成功issuer / migration Readyも未成立である。

## 候補の比較

| 候補 | 判断 |
| --- | --- |
| 別の既定無効境界でrefresh-only保存planを厳密検証し、2属性だけをStateへ反映 | 将来の第一候補。実serializer検証・具体的な許可・本人のEnvironment承認等がすべて必要 |
| State JSONを直接編集しpushする | 採用しない。Terraformの保存planと検証経路から外れる |
| 通常planでcomputed driftを無視する | 採用しない。既存strict drift0を維持する |

refresh-onlyはremote workloadの通常変更を意図しなくても、Stateの更新である。
元のunexpected drift STOPを、包括承認だけからState変更許可へ読み替えない。
このPRは承認可能な具体案を用意するだけで、運用workflow・gate・既存issuer契約を変更しない。

## 試作品が確認する範囲

[`terraform_history_observation_sync_prototype.py`](../../../.github/scripts/terraform_history_observation_sync_prototype.py)は、
通常観測planと仮想のraw State前後を引数で受け取るだけの検証部品である。ファイル・networkを扱わない。
contextは別kindの`dev-history-observation-sync-prototype`とし、観測source 8616 / dev /
Terraform 1.16.4 / Google 8.5.0 / 現dev provider lock hash / offline-prototypeに完全固定する。
ここでのsource SHAは観測入力の出所であり、この試作品を統合した新consumerの実行admissionではない。

- 通常planはcomplete、errored=false、exact12管理対象のaddress / type / provider / modeが一致し、
  全resource changeが値・mask不変のno-op、全outputも既知値のno-opであることを要求する。
  moved / import / replacement / unknown / sensitivity変更、未知・不足・重複graph、deferred actionを拒否する。
- driftはexact history Tableの1件に限り、knownな`etag`と`last_modified_time`の両方だけが変わる。
  etagは非空string、時刻は非bool整数かつ増加とし、null・欠落・型差・片方だけ・逆方向・別fieldは拒否する。
- driftのbefore全attrsとraw State before、driftのafter全attrsとno-op history before / after、
  raw State afterを型込みで完全joinする。planがA→Bなら、別writerによるA→Cを受理しない。
- Stateはexact12、同じlineage・provider・schema version・instanceを保つ。
  許容する差分候補は該当Tableの2leafとserialの**正確な+1**だけ。
  全root / resource / instance / outputs / dependencies / identity / checks / opaque provider-private文字列を比較する。
  dict key順だけを正規化し、resource・instance・配列の順序は変えない。
- raw JSONのduplicate key、NaN / Infinity、非JSON型、boolと整数の混同、範囲外整数、
  深さ・node・本文の上限超過を拒否する。例外は固定reasonだけで、値や識別子を返さない。

serial +1は保守的な仮説上の差分制限で、実Terraformがその形で永続化することやPUT回数を証明しない。
比較はJSON意味上の全要素一致で、State全体のraw byteが不変という主張でもない。
元S3 byte / VersionId / HEADと、変更後の新byte / VersionIdの実証は後段で別途必要になる。

結果kindは既存issuer / receiptと別にし、`prototype_checks=PASS`でも次をfalseに固定する。

- `execution_permitted`
- `native_refresh_only_serialization_proven`
- `full_configuration_verified`
- `fresh_evidence_verified`

既存11件のconfigやcanonical8 schemaの完全な正しさ、checksのsource catalog全体、
planのconfiguration / variables / prior_state / planned_valuesの意味は、この部品では検証していない。
それらはサイズとJSON形を制限するだけである。Stateのchecks / opaque private等は**差分がないこと**を比較する。
既知の現Stateやfresh metadataを読んだとは扱わず、実行時の既存全validatorやprovenanceに代用しない。

## Nativeで未証明の境界

公開binary・架空credentialによる限定mockでrefresh-only plan / mock applyは通ったが、
computed overrideが反映されずdrift0 / State値不変だった。test_planにはschema関連の3 keyだけがあり、
実saved-planのprior_state / planned_values / resource_drift1を観測できなかった。
test_stateにもserial / lineageがなく、2leafだけの実永続化を証明していない。

そのため`validate_refresh_only_candidate`は推測したparserを持たず、入力に関係なく
`UNPROVEN_REFRESH_ONLY`で拒否する。mock成功を実保存plan互換やState変更の許可へ昇格させない。
nativeの安全な検証と独立レビューが完了するまでは、実workflowへの接続も実行も禁止する。

## 将来の実行契約に必要な条件（未実装）

1. 新しい専用source・typed許可・期限をレビューし、exact SHAと実Environment本人承認を結ぶ。
   通常baselineの既存admissionやissuer型を流用せず、別default-off gateからのみ到達させる。
2. fresh full metadata、通常plan、元S3 State本文・全HEAD・VersionId、lineage / serial、
   lock / workspace不存在、既知writer静止を同一の短いwindowで確認する。
   canonical8と既存全12件は既存の厳密validatorで検証し、追加・欠落・未知やsource不一致でSTOPする。
3. 実refresh-only saved-planのserializerを検証して固定し、通常観測A→B・candidate・元Stateを完全joinする。
   native candidateの形やState privateの一般変換を推測で許容しない。元planからexpected drift1を消さない。
4. 実行直前も同SHA、元State VersionId / HEAD / byte、metadataと2属性の期待値、lock / writer / windowを再照合する。
   別writerで値が再変化したら、新しい値へ自動追随せずSTOPする。
5. 将来の実行は保存planに対するCLI **1 invocation**を上限候補とし、native内部retry / PUT回数は未証明と記す。
   unknown / failure時の自動再送・unlock・全snapshot rollbackを追加せず、partial結果をprivateに記録する。
6. 永続化後に全State要素と2leaf / serial、metadata / lock / cleanupを再確認する。
   同期に成功しても、別の通常baselineでimport0 / no-op12 / drift0を実測・独立確認するまで新issuerを発行しない。

継続writerが動く間に通常baselineのdrift0を達成できる保証はない。
この案の実装可否と具体的な追加許可は、未証明箇所を解決したreview可能な差分を根拠に別途判断する。
