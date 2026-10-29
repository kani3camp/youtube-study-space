# DEV BackendをCIで配備する

対象は`test-youtube-study-space` / `asia-southeast2` / `mypage-api`のイメージ更新である。DEV専用WIFを使い、従来の所有者による操作をCIへ移す。本番、実OAuth・認証済みE2E、Privacy・GA4・公開条件の完了を意味しない。

## 配備手順

1. アプリ変更をMyPage統合ブランチへレビュー・統合し、既存CIの`operation=mypage-image-build` / `environment=development`でオフライン検査済みイメージをpushする。実行SHAは選択refの`github.sha`を使い、手入力しない。
2. 実装だけを変更する別PRで、[runtime-release.json](../../../infra/mypage-platform/environments/dev/runtime-release.json)のビルド元SHAとダイジェストを実結果に固定する。前ダイジェストは現在配信中の値とし、ホストは既存値を維持する。配備元とビルド元の`system/`ツリー全体の一致、不変タグからダイジェストへの対応をCIが検証する。
3. 統合後、既存 **CI** を `integration/mypage-canon-20261006` から `operation=mypage-backend-release` / `environment=development` で手動実行する。PR/main/別branch/別actor/再実行/SHA不一致は拒否する。
4. Repository/System検査の成功後、専用Environmentの短命トークンで、Cloud Run v1の元resourceVersionを保持したPUTを1回だけ行う。v2のetagだけを競合防止と扱わない。直前のリビジョン・UID・世代・前ダイジェスト、Hostingバージョン、統合HEADを照合し、イメージ以外の全設定を保持する。
5. 新Readyリビジョン、generation+1、最新への100%トラフィック、v1/v2全設定の不変を確認する。Hostingの匿名APIが`APP_CHECK_REQUIRED` / 403、直接RunがHost検証の`INVALID_REQUEST` / 400をそれぞれno-storeで返し、Hostingの`/`と`/mypage`を配信することを確認する。実利用者のトークンやデータは使わない。
6. 成功時だけ、既存Foundationワークフローの`runtime-public-apply`を同じCI内で実行する。独立したPlan/Apply Environment、HMAC、保存済みPlan、限定的な再取得・State再読み取り・後片付けを維持する。Terraformの一般更新権限やdrift許容は追加しない。
7. `mypage-backend-verified-record`とCI Gateの成功まで確認する。State照合の失敗もCI失敗とし、配信が旧版に戻ったとは扱わない。

ダイジェストをコードレビューで固定するPRは残す。1回のpushでフロントエンド・バックエンド・IaCを一括自動配備する構成ではない。バックエンドを先、フロントエンドを後にする[既存の互換性規則](README.md)を維持する。

## 初回接続

[初期設定スクリプト](../../../infra/mypage-platform/scripts/backend_identity.py)は、引数なしならプレビューだけを行い、`--apply`で以下を追加する。既存名があれば停止する。ソースレビューとCI検証後に接続し、実再読み取りと配備実行を記録する。途中失敗後は現在状態を確認し、無条件に再実行しない。

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

Hostingとバックエンドはリリースの同時実行制御を共有する。実行中は所有者や別経路からの更新を避ける。Cloud RunのresourceVersionはCloud Run内の競合を拒否するが、Hostingやブランチを含むアトミック切替は保証しない。直前・配備後の比較に失敗したら停止する。

変更前に14日保存の`mypage-backend-release-record`を作り、前リビジョン・ダイジェスト、今回のダイジェスト・ビルド元SHA、Hostingバージョン、実行情報を記録する。応答消失は`publishStatus=unknown`とする。Secret値・全環境変数・API応答全文・State・Planは成果物に出さない。State確認成功時だけ、別の検証済み記録を`stateReadback=passed`にする。

自動再試行・ロールバックは行わない。失敗後は現在のリビジョン・ダイジェスト・Hostingバージョン・Stateを読み取り専用で照合する。今回のダイジェストが配信中なら、バックエンド配備は再実行しない。元の実行も再実行せず、疎通確認や競合の原因を直し、必要な既存`runtime-public-plan` / `runtime-public-apply`を新しい実行で行う。State検査が許可しない差は無視しない。ロールバック時はフロントエンドの互換性を確認し、旧イメージを明示した別リリース宣言をレビューして同じCIで配備する。ユーザーデータやSecretは巻き戻さない。
