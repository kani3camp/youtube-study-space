# MyPage OAuth transaction TTL

[Data03](https://app.notion.com/p/3ec357a8d0ce81ac94eef9e6ac0c9964)の `oauth-transactions/expiresAt` TTLを管理するMyPage専用追加レイヤー。既存Firestore databaseのfield一つを、[dev](environments/dev/main.tf)・[prod](environments/prod/main.tf)の独立root/stateで管理する。従来GCP Terraform移行のimport-only gate・state・workflowへ追加しない。

`expiresAt` は既存アプリが作成時刻+10分のTimestampとして保存する。[applicationの期限判定](../../system/core/mypage/auth.go)は維持する。Firestore TTL削除は非同期であり、期限到達時の即時削除や認証の失効を保証しない。[Firestore TTL](https://docs.cloud.google.com/firestore/native/docs/ttl)

## Scopeと停止条件

[共有module](modules/oauth-transaction-ttl/main.tf)は `google_firestore_field` 一つ、`ttl_config {}`、offsetなし。database/API/IAM/rules/authの作成・変更は含まない。`index_config` は省略し、継承indexを維持する。既存の明示index overrideがあればplan検査で停止し、別途reviewする。空の `index_config {}` はindexを無効化するため追加しない。[固定provider仕様](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/website/docs/r/firestore_field.html.markdown)

`prevent_destroy` と `deletion_policy = "PREVENT"` を設定する。撤去・置換・TTL無効化はこの手順に含めない。support三collectionの保持/削除、新D01方式の実装、実support completion、実環境のindex/運用は別のrelease gateのまま。CI成功は公開承認やTTLのlive ACTIVE証拠ではない。

## Credential不要の検証

Terraformの固定版は[.terraform-version](.terraform-version)、providerは各rootのrequired_providersとlockfileを使う。repository rootから:

```sh
bash infra/mypage-oauth-ttl/scripts/check-local.sh
```

このscriptはbackendを初期化せず、provider download・validate・mock_providerによるplan・plan contract回帰・docs/routingを検証する。Terraform testは全runでmockを使い、applyや実Google API呼出しをしない。CIは `contents: read` のみで、OIDC・cloud credential・認証workflowを使わない。provider downloadのnetworkは必要。例のproject/databaseは合成fixtureで、liveへ流用しない。

## 限定plan contract

[plan_contract.py](scripts/plan_contract.py)は `terraform show -json` のplanと独立したowner-approved target manifestを読む。固定resource address/type/provider、environment/project/database、TTL有効、offsetなし、index不変を照合する。create・TTL有効化だけのupdate・no-opを許可し、delete/replace/import/move/drift/deferred/追加resource/data source/unknown target/sensitive targetを拒否する。provider computedのid/nameはcreate時、TTL stateと省略offsetのunknownはcreate/TTL有効化時に限って扱う。

```sh
python3 infra/mypage-oauth-ttl/scripts/plan_contract.py \
  --plan /private/mypage-dev.plan.json \
  --target /private/mypage-dev-target.json
```

出力は固定status/code、plan/target SHA256、actionだけ。入力値・path・raw例外をechoしない。成功時も `executionAuthorized=false`, `releaseReady=false`。この検査はplan JSONの構造・差分の限定検査であり、入力JSONの真正性、state ownership、既存permission、実APIの状態、独立レビューを証明しない。raw plan/stateはsecretを含み得るためpublic artifact/PRへuploadしない。

## Release順序（実行は別途承認）

1. 従来GCP TerraformをMyPage抜きで完了し、ownerがdevelopment branchへ統合する。その確定headを既存MyPage integrationへ通常mergeして独立reviewする。このPRは現在のMyPage integrationに対する追加差分で、dev/prodへ直接mergeしない。
2. MyPage Ready Gateと対象project・既存database・state ownershipを確認する。MyPage rootのbackend keyはdev/prod相互および従来移行と分離する。既存stateで同じfieldを管理していれば停止。private backend設定・tfvars・target manifestを独立した承認済みinventoryから準備し、target manifestのenvironmentもrootと一致させる。例のbucket/regionはplaceholder。認証/権限不足はSTOPであり、このPRでIAM/authを変更しない。
3. developmentの操作一覧・対象・plan・rollback・synthetic observationをownerが承認した後に限り、承認済みoperatorがprivate backendで `terraform -chdir=infra/mypage-oauth-ttl/environments/dev init -backend-config=/private/dev-backend.hcl -lockfile=readonly` を行う。実際のfield/TTL/index管理者・既存stateをread-only照合する。既存管理policyはowner間のimport/state移管計画を別途reviewし、このcontractでimportを通さない。既存expired documentの削除影響も確認する。
4. `terraform -chdir=infra/mypage-oauth-ttl/environments/dev plan -var-file=/private/dev.tfvars -out=/private/dev.tfplan`、`terraform -chdir=infra/mypage-oauth-ttl/environments/dev show -json /private/dev.tfplan > /private/dev.plan.json`、上記contract検査を行う。raw planと固定digestをprivate証跡へ残す。independent reviewerがexact commit・locked provider・backend/target・plan hash・index不変を確認する。contract PASSだけでapplyしない。
5. applyの別途承認後に同一saved planを `terraform -chdir=infra/mypage-oauth-ttl/environments/dev apply /private/dev.tfplan` で適用する。plan/state/対象に変化があれば再plan・再review・再承認。API operation完了とTTL `ACTIVE` をread-only確認する。承認済みsynthetic OAuth transactionの正しいTimestampと10分境界のapplication拒否、自然削除を観測し、時刻・target・field・状態・結果をprivate release recordへ記録する。既存実ユーザーdataをprobeに使わない。mock PASSやTTL ACTIVEだけで自然削除をPASSにしない。
6. development観測後、production用の独立backend/target/planとowner承認で同じ順序を繰り返す。prodの観測が済むまで両環境完了とはしない。自然削除の遅延/失敗はpending/STOPとして扱い、アプリの期限判定は弱めない。

TTL削除済みdocumentはTTL無効化やcode rollbackでは復元しない。このレイヤーの通常rollbackとしてdestroy/TTL無効化を実行しない。異常時は後続release/観測を停止し、ownerが影響とforward-fix/復旧を別途承認する。元の移行gateを緩めない。[MyPage release gate](../../docs/mypage/release/README.md)・[release record](../../docs/mypage/release/release-record.example.json)にexact commit、private plan/observation参照、未完gateを記録する。
