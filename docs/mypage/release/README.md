# MyPage release preparation

このrunbookはインフラ待ち・本人判断待ちを残したまま、コード・設定・試験の準備を進める入口。設定検査の成功は公開承認やlive E2Eの成功ではない。見た目・Phase 1のread-only scope・既存identity/data契約は維持する。

プロダクト方針の正本は[Current Canon](https://app.notion.com/p/3873eac14a174f14881a18eb389f786c)、[判断記録D01〜03](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f)。wire/実装は[実装contract](../README.md)、[OpenAPI](../openapi.yaml)を正とする。本人への準備事項は[owner packet](owner-packet.md)、実経路の試験・release対・rollbackは[E2E手順](e2e.md)にまとめる。

## 1. 現在ある実装と、残る完了条件

| 領域 | 既存実装・localで検証できること | 公開前に残ること |
| --- | --- | --- |
| OAuth / identity | [AuthService](../../../system/core/mypage/auth.go)、atomic claim/consume、channel確認、短命token、purpose分離 | 実Google account/Brand Account/channel選択・審査 |
| OAuth transaction TTL | [MyPage専用Terraform](../../../infra/mypage-oauth-ttl/README.md)、dev/prod定義、credential不要mock/限定plan contract | owner-approved target/state、別途承認したplan/apply、両環境のTTL ACTIVE・自然削除観測。applicationの10分失効は維持 |
| API / Firebase | [server assembly](../../../system/core/mypage/server.go)、keyless SDK、署名/claim、Origin、6 endpoints | 実App Check/Auth、runtime signer/IAM、Hosting経路/Host |
| metadata | refresh、正常なchannel不在のclear、29日cleanup、23時間heartbeat判定 | scheduled runner/期限対応sink/alertの実結線、実inventory、自然実行 |
| 作業集計 | Firestore snapshot/readTime、JST境界、小数秒fixture、欠損/partial | history coverage・writer atomicity・query indexの実証跡 |
| ServiceAccessControl | [MyPage guardとtrusted moderation CLI](../service-access-control.md)、独立reasonとtransaction/checkpoint、合成QA | 実environmentの権限・runtime・運用結線、全runtime drainと横断削除B |
| support | request/purpose/channel binding、fresh proof、opaque参照、[demo限定dry-run](../../../system/cmd/mypage-operator-dryrun/main.go) | 実受付と返信運用、実environmentのCLI結線、横断削除/revoke/開示の実行・検証 |
| public policy | anonymous routes、明示同意、公開情報差込とdraft文面 | 運営値・実送信/保持との照合、本文承認、D02の規約適合性確認 |
| GA4 | consent世代、同意前非送信、撤回/cross-tab、safe固定UTM、loader | Console/全tag inventory・実Network・保持設定 |
| Hosting | [pure候補](../../../mypage/src/hosting.ts)、明示CSP inventory、API優先rewrite、asset検査 | 所有target、実SDKのinventory、callback/access-log、実security headers |
| Bot入口 | 正本で `!app`/alias・固定public入口/UTMを決定済み。設定・受入条件は本runbookで準備 | runtimeコマンド接続は本PRに含めない。別の実装scope承認、Phase 1の実URL設定・live導線/流入確認 |
| 表示・操作 | visual/runtime/privacy/font/bfcache合成QA | 実端末・実account差。受容済みMyPage見た目を再設計しない |

support proofやdry-runは削除実行ではない。古い `mypage-users` mapping向け運用コードを、channel UIDの現在契約へ確認なしで流用しない。metadata cleanupはinjectable jobまでで、schedulerや監視sinkが作成済みとは扱わない。

## 2. 外部接続なしの設定preflight

public設定は[Vite/Firebase/GA4](../../../mypage/src/firebase.ts)、server設定は[entrypoint](../../../system/cmd/mypage-server/main.go)が使う既存契約へ揃える。設定値の元は選択済みproject/appの管理画面・承認済みinventoryであり、例の値を実projectへ流用しない。

```sh
# repository root: explicit public-only JSONだけを読む。SDK/ADC/通信なし。
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
# private manifestをstdinで渡してもよい。server環境全体をexportしない。
node mypage/scripts/check-release-config.mjs --stdin < /path/to/private-public-config.json

# system: 現在processの設定を既存validatorで確認する。ADC/listen/通信なし。
cd system
go run ./cmd/mypage-server --check-config
```

backend検査だけはserver-only secretを既存の安全なprocess環境から読む。コマンド引数やmanifest/Viteへsecretを移さない。出力は固定status/field/codeのみ。public checkerは64KiB以内の明示JSONを読み、秘密・debug/未知field、project/app/origin/同意版の不一致を拒否する。optional backend companionは非秘密の識別・版だけ、optional Hostingは既存candidateのvalidatorを再利用する。

manifestのtop levelは `schemaVersion: 1`, `deployment`, `frontend`、任意の `backend`, `hosting`, `publicPolicy`。`deployment`/`backend`は `environment`, `projectID`, `projectNumber`, `webAppID`, `publicOrigin`, `privacyVersion`, `termsVersion` の7項目。`frontend`の正確なkeyは[例](public-config.example.json)と[public schema](../../../mypage/src/release-config.ts)を使い、server環境をコピーしない。`publicPolicy`は `VITE_PUBLIC_CONTACT_URL`, `VITE_PUBLIC_PRIVACY_CONTACT_URL`, `VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE`, `VITE_PUBLIC_TERMS_EFFECTIVE_DATE`, `VITE_PUBLIC_JURISDICTION`、任意の `VITE_PUBLIC_OPERATOR_NAME`。`hosting`は[既存HostingInputs](../../../mypage/src/hosting.ts)の明示inventoryを使う。optional入力の省略は `not-supplied` として未検証のまま残す。

両検査とも外部登録・keyの真正性・permission/quota・公開本文承認を確認しない。`configurationValid=true`/`configuration=valid`でもinfrastructure/live E2Eはpending。`MYPAGE_INFRASTRUCTURE_READY=true`は起動に必要な宣言であり、Gateの証拠ではない。prepare時の検査のためにlive起動・Ready flag変更を行わない。

### 設定の受け渡し

| boundary | 項目 | 受け渡し・検証 |
| --- | --- | --- |
| public browser | `VITE_FIREBASE_API_KEY/AUTH_DOMAIN/PROJECT_ID/APP_ID`, `VITE_APP_CHECK_SITE_KEY` | 公開Web app設定。secret/admin key/debug tokenではない。project番号とapp、登録domainは別途照合 |
| 同意版 | browser `VITE_PRIVACY_POLICY_VERSION/VITE_TERMS_VERSION`; server `MYPAGE_PRIVACY_VERSION/MYPAGE_TERMS_VERSION` | 同じrelease対で一致。実際に承認された本文の版を記録 |
| origin | `MYPAGE_PUBLIC_ORIGIN`; analytics用 `VITE_MYPAGE_PUBLIC_ORIGIN` | environmentの固定HTTPS origin。Host/Originをclient headerから推測しない |
| server identity | `MYPAGE_ENVIRONMENT/REGION/PROJECT_NUMBER/WEB_APP_ID/SIGNER_EMAIL`, `GOOGLE_CLOUD_PROJECT` | development=asia-southeast2、production=asia-northeast2。signerは選択projectのもの |
| server secret | `MYPAGE_OAUTH_CLIENT_ID/CLIENT_SECRET`, optional `MYPAGE_YOUTUBE_API_KEY` | OAuth secret/API keyは承認済みsecret供給境界。repo/chat/public artifact/Viteに入れない |
| optional Analytics | `VITE_MYPAGE_ANALYTICS_READY`, `VITE_MYPAGE_GA4_ID` | 実Console・network inventoryが承認されるまで有効化しない。Consentを迂回しない |
| 公開案内 | `VITE_PUBLIC_CONTACT_URL/PRIVACY_CONTACT_URL`, optional `VITE_PUBLIC_OPERATOR_NAME`, `VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE/TERMS_EFFECTIVE_DATE/JURISDICTION` | 公開に使ってよい値だけ。実名・自宅住所を必須にしない。欠落/不正は未設定、draftは維持 |
| Bot設定案 | `MYPAGE_PUBLIC_ENTRY_URL`（runtimeへの接続は未実装） | 承認済みpublic入口だけ。user固有query/tokenを含めない。0A/0Bは未設定、Phase1承認後に設定。実装・設定の承認前に有効化しない |

publicPolicy入力は一般問い合わせとPrivacy請求を分ける。形式が正しいURLでも返信機能・ログイン不要・SLA運用が確認済みとは限らない。

## 3. 再利用するlocal acceptance

依存は各packageのlock/toolchain設定を使う。秘密や実ユーザーdataは不要。以下はrepository-nativeの確認で、real OAuth/Hostingを代替しない。

```sh
# root
python3 .github/scripts/check-mypage-contract.py
python3 .github/scripts/check-doc-references.py
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
```

各packageの確認は、それぞれrepository rootから開始する。

```sh
cd system
go test -shuffle=on ./...
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
```

```sh
cd mypage
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

Go変更の必須checksは[AGENTS.md](../../../AGENTS.md)。生成後の意図しない差分0を確認する。Firestore依存のacceptanceは[既存Emulator script](../../../.github/scripts/run-firestore-integration-tests.sh)をrootから実行する（demo project、real credentialを渡さない）。

browserはlocal Viteをport18081で起動し、[runtime](../../../mypage/scripts/runtime-qa.py)・[privacy](../../../mypage/scripts/privacy-qa.py)・[visual](../../../mypage/scripts/visual-qa.py)・[font](../../../mypage/scripts/font-qa.py)を使う。各scriptは外部requestを遮断する。実bfcacheは[専用static build](../../../mypage/scripts/bfcache.vite.config.ts)と[既存QA](../../../mypage/scripts/bfcache-qa.py)を使い、dev HMRを証拠にしない。設定済みpublic文面は専用testとbrowserで匿名到達/安全なlinkを確認する。実ユーザー画面をpublic QA artifactへ保存しない。

## 4. Gateと承認の順序

| checkpoint | 完了を示す証跡 | 未完時 |
| --- | --- | --- |
| MyPage Ready Gate | [infra正本Phase6](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)のdev/prod基盤・ownership・Firebase境界・keyless CI | resource本実装/provisioningへ進まない |
| Phase 0A準備 | exact commit/CI、設定preflight、公開文面とAPI前同意、対象/rollback/実試験計画 | local prepareは継続。target値を推測しない |
| 0A実施承認 | 対象project/domain/revision、操作一覧、本人test accounts、infra/security証跡、秘密の供給方式 | live実行/公開/追加IAMを行わない |
| 0B受入 | 0A実経路PASS、D01実装証跡、D02規約適合性確認、D03実値、返信窓口、7日削除/cleanup、実metadata/log/backup inventory | 実ユーザー受入を停止 |

Privacy受付の[アプリ内source package](../privacy-intake.md)はdefault off。公開manifestの`VITE_PRIVACY_INTAKE_ENABLED`は現時点で`false`のみ受理し、server側も`EnablePrivacyIntake=false`を維持する。返信担当/認可/監査とログイン不能時の人的例外窓口を成立させ、D02の規約適合性・D03の実値と実経路を確認してから別の公開判断を行う。
| Phase 1一般公開 | OAuth verification完了、0B実account/端末QA、Bot/外部導線、rollback、monitoring | 日付やCI成功だけで告知しない |

正常session revokeで最大約1時間の残存を許容する既定を、削除時の全writer/cache静止の代替にしない。D01方式は2026-10-07 Canonの独立ServiceAccessControlに決定済み。AのguardとBの横断削除/drain完了を区別する。D02は2026-10-09に外部Google解除検知だけを目的とするrefresh token保存・定期pollingを採用しないと確定し、規約適合性と取消後の保持済みAPI dataの扱いをverification/complianceで確認する。

## 5. 証跡と停止条件

[release record例](release-record.example.json)にfrontend commit/Hosting release、backend image digest/revision、rewrite/pinTag、schema/同意版、試験のscopeと参照、owner承認をひと組で記録する。例はD02設計選択のみ確定済みとし、残るGateはpendingのtemplateで、入力しただけではGateを閉じない。private target/個人data/credentialをpublic PRに載せない。

認証/IDOR・誤channel・code/state漏えい・同意前GA4・古いdata復活・保持/削除期限違反・rollback互換不成立はSTOP。部分失敗はunknown/pendingとして記録し、再実行で失敗を消さない。CodeQLの既存OIDC custom-claim問題とGitHub CI Gateは区別し、解析未開始をsecurity scan PASSとは呼ばない。OIDC/IAM変更はこのrunbookのコマンドには含めない。
