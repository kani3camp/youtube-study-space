# MyPage OAuth transaction TTL

[Data03](https://app.notion.com/p/3ec357a8d0ce81ac94eef9e6ac0c9964)の`oauth-transactions/expiresAt` TTLを管理するMyPage専用の追加設定である。既存Firestore DBの1フィールドを、[dev](environments/dev/main.tf)・[prod](environments/prod/main.tf)の独立したルートとStateで管理する。従来GCP Terraform移行のimport-only検査・State・ワークフローには追加しない。

`expiresAt`は、既存アプリが作成時刻から10分後のTimestampとして保存する。[アプリの期限判定](../../system/core/mypage/auth.go)は維持する。Firestore TTL削除は非同期であり、期限到達時の即時削除や認証失効を保証しない。[Firestore TTL](https://docs.cloud.google.com/firestore/native/docs/ttl)を参照する。

## 対象範囲と停止条件

[共有モジュール](modules/oauth-transaction-ttl/main.tf)は、1つの`google_firestore_field`と`ttl_config {}`を使い、offsetを指定しない。DB・API・IAM・ルール・認証の作成や変更は含めない。`index_config`を省略して継承インデックスを維持する。既存の明示的なインデックス上書きがあればPlan検査で停止し、別途レビューする。空の`index_config {}`はインデックスを無効にするため追加しない。[固定プロバイダー仕様](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/website/docs/r/firestore_field.html.markdown)を参照する。

`prevent_destroy`と`deletion_policy = "PREVENT"`を設定する。撤去・置換・TTL無効化はこの手順に含めない。サポート3コレクションの保持・削除、新D01方式の実装、実サポート受付の完了、実環境のインデックスと運用は別の公開条件として残す。CI成功は公開承認や実TTLのACTIVE状態の証拠ではない。

## 認証情報を使わない検証

Terraformの固定バージョンは[.terraform-version](.terraform-version)、プロバイダーは各ルートのrequired_providersとロックファイルを使う。リポジトリのルートから実行する。

```sh
bash infra/mypage-oauth-ttl/scripts/check-local.sh
```

このスクリプトはバックエンドを初期化せず、プロバイダーの取得・validate・mock_providerによるPlan・Plan仕様の回帰を検証する。Terraform testは全実行でモックを使い、applyや実Google API呼び出しは行わない。CIは`contents: read`だけを使い、OIDC・クラウド認証情報・認証ワークフローは使わない。プロバイダー取得のネットワークは必要である。例のプロジェクト・DBは合成テストデータであり、実環境へ流用しない。

## Planの差分を限定する検査

[plan_contract.py](scripts/plan_contract.py)は `terraform show -json` のplanと独立したowner-approved target manifestを読む。固定resource address/type/provider、environment/project/database、TTL有効、offsetなし、index不変を照合する。create・TTL有効化だけのupdate・no-opを許可し、delete/replace/import/move/drift/deferred/追加resource/data source/unknown target/sensitive targetを拒否する。provider computedのid/nameはcreate時、TTL stateと省略offsetのunknownはcreate/TTL有効化時に限って扱う。

```sh
python3 infra/mypage-oauth-ttl/scripts/plan_contract.py \
  --plan /private/mypage-dev.plan.json \
  --target /private/mypage-dev-target.json
```

出力は固定のstatus/code、Plan・対象のSHA256、actionだけとする。入力値・パス・元の例外は表示しない。成功時も`executionAuthorized=false`、`releaseReady=false`とする。この検査はPlan JSONの構造と差分を限定するもので、入力JSONの真正性、Stateの所有権、既存権限、実API状態、独立レビューを証明しない。元のPlan/Stateはシークレットを含み得るため、公開成果物やPRにアップロードしない。

## Release順序（実行は別途承認）

1. 従来GCP TerraformをMyPage抜きで完了し、ownerがdevelopment branchへ統合する。その確定headを既存MyPage integrationへ通常mergeして独立reviewする。このPRは現在のMyPage integrationに対する追加差分で、dev/prodへ直接mergeしない。
2. MyPage Ready Gateと対象project・既存database・state ownershipを確認する。MyPage rootのbackend keyはdev/prod相互および従来移行と分離する。既存stateで同じfieldを管理していれば停止。private backend設定・tfvars・target manifestを独立した承認済みinventoryから準備し、target manifestのenvironmentもrootと一致させる。例のbucket/regionはplaceholder。認証/権限不足はSTOPであり、このPRでIAM/authを変更しない。
3. developmentの操作一覧・対象・plan・rollback・synthetic observationをownerが承認した後に限り、承認済みoperatorがprivate backendで `terraform -chdir=infra/mypage-oauth-ttl/environments/dev init -backend-config=/private/dev-backend.hcl -lockfile=readonly` を行う。実際のfield/TTL/index管理者・既存stateをread-only照合する。既存管理policyはowner間のimport/state移管計画を別途reviewし、このcontractでimportを通さない。既存expired documentの削除影響も確認する。
4. `terraform -chdir=infra/mypage-oauth-ttl/environments/dev plan -var-file=/private/dev.tfvars -out=/private/dev.tfplan`、`terraform -chdir=infra/mypage-oauth-ttl/environments/dev show -json /private/dev.tfplan > /private/dev.plan.json`、上記contract検査を行う。raw planと固定digestをprivate証跡へ残す。independent reviewerがexact commit・locked provider・backend/target・plan hash・index不変を確認する。contract PASSだけでapplyしない。
5. applyの別途承認後に同一saved planを `terraform -chdir=infra/mypage-oauth-ttl/environments/dev apply /private/dev.tfplan` で適用する。plan/state/対象に変化があれば再plan・再review・再承認。API operation完了とTTL `ACTIVE` をread-only確認する。承認済みsynthetic OAuth transactionの正しいTimestampと10分境界のapplication拒否、自然削除を観測し、時刻・target・field・状態・結果をprivate release recordへ記録する。既存実ユーザーdataをprobeに使わない。mock PASSやTTL ACTIVEだけで自然削除をPASSにしない。
6. development観測後、production用の独立backend/target/planとowner承認で同じ順序を繰り返す。prodの観測が済むまで両環境完了とはしない。自然削除の遅延/失敗はpending/STOPとして扱い、アプリの期限判定は弱めない。

TTL削除済みdocumentはTTL無効化やcode rollbackでは復元しない。このレイヤーの通常rollbackとしてdestroy/TTL無効化を実行しない。異常時は後続release/観測を停止し、ownerが影響とforward-fix/復旧を別途承認する。元の移行gateを緩めない。[MyPage release gate](../../docs/mypage/release/README.md)・[release record](../../docs/mypage/release/release-record.example.json)にexact commit、private plan/observation参照、未完gateを記録する。
