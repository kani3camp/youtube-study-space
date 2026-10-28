# DEV BackendをCIで配備する

対象は `test-youtube-study-space` / `asia-southeast2` / `mypage-api` のイメージ更新。DEV専用WIFで既存owner操作をCIへ移す。Production、実OAuth/認証済みE2E、Privacy/GA4/公開Gateの完了を意味しない。

## 配備手順

1. アプリ変更をMyPage統合ブランチへレビュー・統合し、既存CIの `mypage_foundation=true` / `mypage_foundation_mode=image-build` / exact SHAでoffline check済みイメージをpushする。
2. 別のsource-only PRで [runtime-release.json](../../../infra/mypage-platform/environments/dev/runtime-release.json) のbuild元SHA/digestを実結果へ固定する。previous digestは現在配信中の値、hostは既存値を維持する。配備元とbuild元の `system/` tree全体の一致、immutable tag→digestをCIが検証する。
3. 統合後、既存 **CI** を `integration/mypage-canon-20261006` から `mypage_backend=true` / `mypage_backend_expected_sha=<統合HEADの40桁SHA>` で手動実行。他の認証操作とHosting配備は全てfalseにする。PR/main/別branch/別actor/再実行/SHA不一致は拒否する。
4. Repository/System成功後、専用Environmentの短命tokenで、Cloud Run v1の元resourceVersionを保持したPUTを1回だけ行う。v2のetagだけを競合防止と扱わない。直前revision/UID/generation/前digest、Hosting版、統合HEADを照合し、image以外の全設定を保持する。
5. 新Ready revision、generation+1、最新100% traffic、v1/v2全設定不変を確認。Hostingの匿名APIが `APP_CHECK_REQUIRED` / 403、直接RunがHost境界の `INVALID_REQUEST` / 400をそれぞれno-storeで返し、Hostingの `/` と `/mypage` が配信されることを確認する。実利用者のtoken/dataを使わない。
6. 成功時だけ既存Foundation workflowの `runtime-public-apply` を同じCI内で実行する。独立Plan/Apply Environment、HMAC、saved Plan、限定refresh/State readback/cleanupを維持する。Terraformの一般update権限やdrift許容を追加しない。
7. `mypage-backend-verified-record` とCI Gate成功まで確認する。State照合失敗もCI失敗。配信が旧版へ戻ったとは扱わない。

digestをコードレビューで固定するPRは残す。1回のpushでFrontend/Backend/IaCを一括自動配備する仕組みではない。Backend先行→Frontend後行の[既存互換性ルール](README.md)を維持する。

## 初回接続

[bootstrap](../../../infra/mypage-platform/scripts/backend_identity.py) は引数なしならpreviewだけ、`--apply` で以下を追加する。既存名があれば停止。Sourceレビュー・CI検証後に接続し、実readbackと配備runを記録する。途中失敗後は現在状態を確認し、無条件再実行しない。

| 追加先 | 権限・設定 |
| --- | --- |
| DEV専用SA `mypage-backend-deploy` | keyなし。既存Terraform identity/State権限は変更しない |
| DEV `mypage-backend` WIF pool/provider | repo/owner/actor固定ID、初回workflow_dispatch、統合ref、caller `ci.yml`、本reusable、同一workflow SHA、専用Environmentと完全subject |
| `mypage-api` serviceだけ | custom role: `run.services.get` / `run.services.update` |
| `mypage-api` runtime SAだけ | `roles/iam.serviceAccountUser`（actAs） |
| DEV `mypage` registryだけ | `roles/artifactregistry.reader`。pushは既存image-build経路 |
| DEV project | `roles/firebasehosting.viewer`（Hosting版比較）、`roles/serviceusage.serviceUsageConsumer`（quota）。Hosting writeなし |
| GitHub `mypage-dev-backend` | integration branchだけ/tagなし。非秘密Variable `MYPAGE_BACKEND_WIF_PROVIDER` |

**IAMの能力**：`run.services.update` はimage以外の設定も変更でき、runtime SAのactAsと組み合わさるとそのSAの権限で任意コードを実行できる。IAMでimage fieldだけに限定したとは呼ばない。固定対象/source/digest/WIF admission/保持比較/CI保護で運用境界を設ける。Secret payload read、IAM setter、Cloud Run create/delete、Production権限をCIへ直接付与しない。公式根拠：[配備権限](https://docs.cloud.google.com/run/docs/reference/iam/roles)、[v1 replaceService](https://docs.cloud.google.com/run/docs/reference/rest/v1/namespaces.services/replaceService)。

## 失敗・復旧

HostingとBackendはrelease concurrencyを共有する。実行中はowner/別経路の更新を避ける。Cloud Run resourceVersionはCloud Run競合を拒否するが、Hosting/branchを含むatomic切替は保証しない。直前・配備後比較が失敗すれば停止。

mutation前に14日保存の `mypage-backend-release-record` を作り、前revision/digest、今回digest/build元SHA、Hosting版、runを記録する。応答喪失は `publishStatus=unknown`。Secret値/env全文/API応答全文/State/Planをartifactへ出さない。State成功時だけ別verified recordが `stateReadback=passed` となる。

自動retry/rollbackは行わない。失敗後は現在revision/digest/Hosting版/Stateをread-only照合する。今回digestが配信中ならBackend配備を再実行しない。元runを再実行せず、smoke/競合原因を直し、必要な既存 `runtime-public-plan` / `runtime-public-apply` を新runで行う。State検査が許可しない差を無視しない。rollbackはFrontend互換性を確認し、旧imageを明示した別release宣言をレビューして同じCIで配備する。ユーザーデータ/Secretを巻き戻さない。
