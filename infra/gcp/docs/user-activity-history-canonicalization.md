# Development user-activity-history: schema repair before adoption

この文書は公開可能な判定・準備仕様です。private metadata、実件数、row、IAM inventory、
state/planは保存しません。入口は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)。

## Recorded audit outcome

[Audit run 37475212059](https://github.com/kani3camp/youtube-study-space/actions/runs/37475212059) は
reviewed SHA `756d32f15d179f2a54d99df9ae43106b100b49e8`、attempt1で成功しました。
認証、aggregate-only operation、sanitized Summary、private output cleanupは成功です。
下記判定の根拠は、このrunのSummaryをユーザーが貼付した内容です。Summaryの本文をAPIで
独立取得できたとは扱いません。

| Sanitized predicate | User-provided Summary |
| --- | --- |
| Legacy timestamp present | yes |
| legacy_non_null is zero | yes |
| legacy_only is zero | yes |
| both_equal is zero | yes |
| both_different is zero | yes |

実装SQLは[現行audit](../../../system/core/mybigquery/user_activity_schema_audit.go)です。
`legacy_non_null`は`timestamp IS NOT NULL`、他3集計はその行を`taken_at IS NULL`、
両値equal、両値differentに分けます。今回の監査時点では非NULLのlegacy値がなく、
legacy値からのbackfill/repairは不要です。tableが空であること、`taken_at`全体の非NULL性・
正確性・重複のなさ、監査後のwriterの挙動はこの判定から保証しません。

[Source closure #1238](https://github.com/kani3camp/youtube-study-space/pull/1238) 後のintegration SHAは
`829b0123174583f3e485dddcac2ddf405b4a093f`、audit gateはfalseです。
runtimeのtemporary trust/bindings/inputs撤回とSA disableはoperatorのreadback報告で完了。
このsource作業でGCP cleanupを独立再実行・再検証したとは扱いません。追加queryなし。

## Decision: preserve the canonical contract

値が空でもlegacy列自体は残っています。Notion Current Canonと#1191のこのtableに対する
具体方針は、schema修復を別approved waveで終えてからcanonical8列をimportする順序です。
一般の「import first」原則を理由に、Unexpected Driftの9列を新しい正本にしません。

現状9列を正確に定義してno-op importする案は技術的に別案ですが、環境別timestamp許容と
module/ownership方針の変更が必要で、現在の合意とは異なります。schema ignoreが必須な案では
ありませんが、ignoreでdriftを隠す案も採用しません。このPRでは9列案を実装しません。
[module](../modules/retained-user-activity-history/main.tf)とdevelopment adoption gateは変更しません。

## Offline preparation

判断packetとexact import/post-planのoffline検査は
[development history readiness](development-history-ready-packet.md)へ集約します。

[prepare_user_activity_history_adoption.py](../scripts/prepare_user_activity_history_adoption.py) は
cloud clientを持たず、privateなcomplete `tables.get` metadataから既存canonical列順だけを
準備します。legacy/unknown/missing/duplicate field、nested mode/type/orderの差、未表現の
description/policy tagやtable設定を拒否します。canonical8列でも出力の
`manage_user_activity_history`はfalseです。metadataの取得元・freshness・caller/trust・
consumer・runtime deployment・完全なprovider no-opはoffline helperでは証明できません。

既存権限内のoperatorが、承認済みschema repair後にexact targetのfresh metadataをprivate
directoryへ保存して使う想定です。owner-only input、重複JSON key拒否、output0600・新規作成
のみで、private値やinput pathをstdout/stderrへ出しません。public CIは合成metadataのみ。

```sh
python3 infra/gcp/scripts/prepare_user_activity_history_adoption.py \
  /private/fresh-table-metadata.json /private/candidate.tfvars.json
```

これは認証付きplan/import/applyを実行するコマンドではありません。

## Separate live waves, all still unapproved

1. **Dependency and recovery preflight**: external BI/cross-project/manual SQL/wildcard/views、
   current export/tmp canonical schema、#1192 guardのactual runtime deployment SHA、writer windowを
   確認。旧auditを現在も非NULL値なしの証明にしない。row access policies、partition/clustering、
   constraints、schema/modes/orderをfresh metadataで確認。短期recoveryの実行経路・期限・
   private retention・新write保持・復元proofが未確定ならSTOP。snapshot/copy/deploy/IAM等が
   必要ならそのexact scopeも別承認。追加business-data queryをこの準備へ自動追加しない。
2. **Schema-only repair**: 上記gateが満たされた時だけ、既存operator identityのpermissionと
   exact targetを確認してDROPのaction-time承認を取る。候補は次の1 statementだけ。
   `IF EXISTS`でschemaの想定外変更を隠さず、retry/backfill/table再作成は追加しない。

   ```sql
   ALTER TABLE `test-youtube-study-space.firestore_export.user-activity-history`
   DROP COLUMN `timestamp`;
   ```

   BigQuery job locationは`asia-southeast2`。DDLにはtable `get/update`とjob作成の権限確認が
   必要です。disabled audit SAを流用・再有効化する指示ではありません。DROP後はfresh
   `tables.get`で、timestampだけが消え、他の8列・nested定義・相対順序・table設定が不変と
   確認。table/row overwrite、列順統一、Terraform state変更は0。
3. **Import-only adoption**: repair後のfresh canonical metadataでoffline inputsを準備。
   actual pinned-provider isolated plan → approved full-root planで、table import1、既存11 no-op、
   create/update/delete/replace/drift/unknown/other0を必要条件とする。これは未実測のacceptance
   targetです。activationは別reviewed source PR、protected applyは別承認。同一SHA re-planと
   既存global import-only guardを維持し、post import0/no-op12/その他0、state/lockをreadback。
   consumer/recovery/ownershipの問題をimport applyへ混ぜない。

## DROP implications and rollback

[BigQuery DDL documentation](https://docs.cloud.google.com/bigquery/docs/reference/standard-sql/data-definition-language#alter_table_drop_column_statement)
は、partition/clustering/constraint/nested/row-policy等の制限、参照view等の別対応、DROP後の
legacy SQL/wildcard/BI Engine/copy制限を説明しています。空の列でもconsumerへの影響は残り、
DROPはstorageの即時消去ではありません。現在のdependenciesと選んだrecovery経路がこれらの
制限と両立するか、既存権限内のprivate reviewで確認する必要があります。

`ADD COLUMN timestamp TIMESTAMP`だけでは旧値・元の列位置は戻りません。table overwriteや
recovery tableへのcopyも無条件のrollbackにせず、新writeとprivacy retentionを守る手順を
対象・期限・権限・cost込みで別承認します。失敗や想定外schema時は以降のimportを停止します。

[Pinned provider source](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/bigquery/resource_bigquery_table.go)
のupdate経路は、remoteにだけ残る列をDDLでDROPしてからtable設定を更新できます。
`deletion_protection`/`prevent_destroy`だけを列削除の防止と扱いません。
9列remoteへ8列definitionを当てたimport+update planは必ず拒否し、repairとimportを分離します。

Schema repair/import、runtime WIF/API ownership、production基盤は未完了です。
audit成功とcode/testsだけでInfrastructure/MyPage release ReadyGateをPASSにしません。
