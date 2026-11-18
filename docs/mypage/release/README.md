# MyPage 配備・公開準備手順

この文書は MyPage の設定事前検査・CI 配備・公開前の確認条件を案内します。メンテナンスモードの初期化・開始・解除や利用拒否からの復旧は [メンテナンスモード v1 の手順書](../maintenance-mode.md) を参照してください（通常リリース・Terraform apply・Hosting 公開とは独立した手動操作であり、同手順書にあるワークフロー以外から Firestore 状態を直接変更しません）。

> [!IMPORTANT]
> 設定検査やローカルの合成 QA、設計決定を実経路の合格や公開承認とはみなしません。実装仕様は [MyPage 実装仕様](../README.md)、API は [OpenAPI](../openapi.yaml)、受入項目と復旧試験は [E2E 手順](e2e.md)、最新の統合状況は [PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)、未完の公開受入は [Issue #1133](https://github.com/kani3camp/youtube-study-space/issues/1133) を参照してください。

プロダクト方針は Current Canon の [04 認証](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)、[05 Privacy](https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf)、[06 Security](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634)、[09 判断記録](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f) を参照します。

---

## 構成と配備の入口

開発・本番とも Firebase Hosting（React/Vite）、`/api/**` の書き換え、Cloud Run Go BFF、Firebase Authentication、Firestore で構成します。Cloudflare はドメイン登録と DNS だけに使い、初期構成に Managed WAF・Cloud Armor・専用外部 LB・Cloud Run Web は含めません。

標準の `run.app` にも直接到達できるため、Hosting と直接 Run の両方で認証・App Check・SAC・Origin・UID 制限・Canon S01/S04 を検証します。公開ドメインは `https://moku.work/` とし、登録の Active 状態・DNS/TLS・OAuth コールバック・本番配備は対象環境を再読み取りして確認します。

| 対象 | 手順と確認境界 |
| --- | --- |
| 基盤・鍵を使わない認証・環境別 State | [MyPage 基盤](../../../infra/mypage-platform/README.md) |
| OAuth トランザクションの TTL | [専用 Terraform](../../../infra/mypage-oauth-ttl/README.md)（アプリ側の 10 分失効と非同期 TTL 削除を区別） |
| バックエンドイメージ更新 | [バックエンド CI](backend-ci.md)（PR 統合後検査 → image-build → digest 検証 → Cloud Run image 配備。Terraform は通常リリースで起動しません） |
| フロントエンド | 本ページの「[DEV Hosting を CI で配備する](#dev-hosting-を-ci-で配備する)」と「[静的配信キャッシュの仕様](#静的配信キャッシュの仕様)」 |
| メタデータ保持・監視 | [削除処理と独立監視](metadata-monitoring.md)（ソース実装と実スケジューラー・出力先接続を区別） |
| 横断削除 | [削除手順](../support-delete.md)・[調査対象](../support-delete-inventory.md)（CLI 実装やローカル検証だけでは実削除運用の受入完了とみなしません） |

---

## 公開設定

### GA4 の環境別セットアップ

MyPage の GA4 には各 Firebase プロジェクトに接続済みの `mypage` 用プロパティと `my-page` ストリームを使用します。DEV では同じ GA4 プロパティ内に `youtube-monitor` ストリームもあるため、プロパティ単位の設定を変更すると両ストリームに影響します。

測定 ID は DEV `G-E31NHW939V` / PROD `G-ZJ1VPVT2L2` を採用し、ストリーム URL は承認済みの環境別公開オリジンに揃えて開発アクセスを本番集計へ送らないようにします（ID の採用は確定していますが、ビルド設定への反映・管理画面設定の受入・実通信と Realtime の検証が終わるまでは実送信の有効化完了とはみなしません）。

- 両プロパティで広告連携・Google Signals・広告パーソナライズを使わず、保持 14 か月と新規活動による保持期間リセット OFF を確認し、SPA 側で手動ページビューを送るため Enhanced Measurement の履歴変更時自動ページビューなど重複送信を無効にします。
- `VITE_MYPAGE_GA4_ID=<その環境の測定 ID>` と `VITE_MYPAGE_PUBLIC_ORIGIN=<その環境の HTTPS origin>` を環境別フロントエンドのリリース設定に必ず含めます（測定 ID は公開値でありシークレット扱いしません。公開マニフェスト整合性検査と Hosting CSP の別途承認が必要）。
- 未接続時は初回の同意バナーを表示せず、既存の v1 同意設定は接続前に取得した選択のため再利用せず v2 へ更新します（GA4 有効化後の初回アクセスで、Google Analytics や Cookie を使うサイト改善のためのアクセス解析を案内し、明示的に同意/拒否を選んでもらいます）。
- DEV から先に実ブラウザで、同意前・拒否中の GA4 通信 0 件、許可後の実イベントと URL 秘匿、拒否変更後の非送信と Cookie、Realtime 表示、CSP、外部送信表示を確認します（ローカル合成テストは実ベンダー動作を証明せず、DEV 結果を本番検証に読み替えません）。

---

## 外部接続なしの設定事前検査

公開設定は [Vite/Firebase/GA4](../../../mypage/src/firebase.ts)、サーバー設定は [起動処理](../../../system/cmd/mypage-server/main.go) の仕様に揃えます。値は選択したプロジェクト・アプリの管理画面や承認済み一覧から取得し、例の値を実プロジェクトへ流用しないでください。

```sh
# repository root: 明示的な公開専用 JSON のみを読む（SDK / ADC / 通信なし）
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
# 非公開マニフェストを標準入力で渡す例（サーバー環境全体は export しない）
node mypage/scripts/check-release-config.mjs --stdin < /path/to/private-public-config.json

# system: 現在プロセスの設定を既存バリデータで確認（ADC / listen / 通信なし）
cd system
go run ./cmd/mypage-server --check-config
```

- バックエンド検査は既存の安全なプロセス環境からサーバー専用シークレットを読みますが、コマンド引数・マニフェスト・Vite へは移さず、固定の `status` / `field` / `code` だけを出します。
- 公開設定チェッカーは 64 KiB 以内の明示 JSON を読み、秘密値・デバッグ値・未知フィールドや、プロジェクト・アプリ・オリジンの不一致を拒否します（任意のバックエンド補助宣言は非秘密情報のみを持ち、Hosting 設定には既存検証処理を再利用）。
- マニフェストの最上位項目は `schemaVersion: 1`、`deployment`、`frontend` と任意の `backend`、`hosting` です。`deployment` / `backend` は `environment`、`projectID`、`projectNumber`、`webAppID`、`publicOrigin` の 5 項目を持ち、`frontend` は [例](public-config.example.json) と [公開スキーマ](../../../mypage/src/release-config.ts)、`hosting` は [HostingInputs](../../../mypage/src/hosting.ts) を参照します（任意入力の省略時は `not-supplied` として未検証のまま残ります）。同意版と公開文面の値はマニフェストに含めず、旧項目（`publicPolicy`、`privacyVersion` / `termsVersion`、`VITE_PRIVACY_POLICY_VERSION` / `VITE_TERMS_VERSION`）は未知フィールドとして拒否します。

両検査とも外部登録・鍵の真正性・権限や利用枠・公開本文の承認は確認せず、`configurationValid=true` / `configuration=valid` でも基盤と実環境の E2E は未完了です（`MYPAGE_INFRASTRUCTURE_READY=true` も起動に必要な宣言であり、確認完了の証拠にはなりません）。

### 設定の受け渡し

Hosting の画像接続先一覧（`img-src` 専用で `script` / `connect` / `frame` は広げない）には、チャンネル画像用の `https://yt3.ggpht.com` と `https://yt3.googleusercontent.com` の両オリジンを明示します（reCAPTCHA 画像だけの一覧は事前検査で拒否。将来別の画像オリジンが返った場合は実配信元を確認して一覧を再レビュー）。

| 設定の用途 | 項目 | 受け渡し・検証 |
| --- | --- | --- |
| public browser | `VITE_FIREBASE_API_KEY/AUTH_DOMAIN/PROJECT_ID/APP_ID`, `VITE_APP_CHECK_SITE_KEY` | 公開 Web app 設定（secret / admin key / debug token 不可）。project 番号と app、登録 domain は別途照合 |
| 同意版 | browser [`public-policy-config.ts`](../../../mypage/src/public-policy-config.ts) の `POLICY_VERSIONS`<br>server [`policy.go`](../../../system/core/mypage/policy.go) の `CurrentPrivacyPolicyVersion` / `CurrentTermsVersion` | 本文と同じくソースの定数で管理し、環境変数やマニフェストでは設定しない。両者の一致は `public-policy-config.test.ts` で検査。本文を改訂して再同意を求めるときは両方の版を上げ、[同意版を変える配備](#同意版を変える配備)の順で反映する |
| origin | `MYPAGE_PUBLIC_ORIGIN`<br>analytics 用 `VITE_MYPAGE_PUBLIC_ORIGIN` | 環境ごとの固定 HTTPS origin（Host / Origin をクライアントヘッダーから推測しない） |
| server identity | `MYPAGE_ENVIRONMENT/REGION/PROJECT_NUMBER/WEB_APP_ID/SIGNER_EMAIL`, `GOOGLE_CLOUD_PROJECT` | `development`=`asia-southeast2`、`production`=`asia-northeast2`。signer は選択プロジェクトのもの |
| server secret | `MYPAGE_OAUTH_CLIENT_ID/CLIENT_SECRET`, 任意 `MYPAGE_YOUTUBE_API_KEY` | 承認済みの秘密情報供給境界から渡し、repo・チャット・公開成果物・Vite に入れない |
| GA4（リリース必須） | `VITE_MYPAGE_GA4_ID`, `VITE_MYPAGE_PUBLIC_ORIGIN` | 環境別の測定 ID と公開オリジン（送信には利用者の同意が必要） |
| 公開案内 | [`public-policy-config.ts`](../../../mypage/src/public-policy-config.ts) の `OPERATOR_NAME`・`POLICY_EFFECTIVE_DATE` | 運営者の公開表記は `らいど開発`。施行日は両文書で共通とし、未設定の間は表示しない。管轄裁判所は特定せず、利用規約で法令の定めに従うと記載 |
| お問い合わせ先 | [`public-policy-config.ts`](../../../mypage/src/public-policy-config.ts) の `contactForms` | 一般・プライバシーのお問い合わせで共通フォームを使用。`VITE_FIREBASE_PROJECT_ID` が `test-youtube-study-space` または `youtube-study-space` の場合だけ環境別固定 URL を表示し、未知・未設定プロジェクトではお問い合わせボタンを表示しない |
| ライブ・メンテナンス告知 | [`youtube-channel.ts`](../../../mypage/src/youtube-channel.ts)（キーは `VITE_FIREBASE_PROJECT_ID`） | DEV は `UChBj4xZePvWyGmTKct5FBcg`、PROD は `UCXuD2XmPTdpVy7zmwbFVZWg` の `/live`・`/posts` を使用（公開トップとマイページで共通）。未知・未設定プロジェクトでは両導線を非表示にしフォールバックせず、変更にはソースレビューと再ビルドを必須化 |
| Bot 設定案 | `MYPAGE_PUBLIC_ENTRY_URL`（runtime 未接続） | 承認済み public 入口のみ（user 固有 query/token 不可）。0A/0B は未設定、Phase 1 承認後に設定 |

### お問い合わせ窓口の設定と受入

`/contact` は使い方・不具合・ご意見の一般窓口と、Google 連携解除後の連絡やデータ削除・開示・訂正・全ログイン解除などのプライバシー窓口を分けて案内します。受付と返信は人的窓口で行い、[本人確認・運用 CLI](../support-operator.md) を利用します。

- 窓口 URL はソースの `contactForms` で Firebase プロジェクトごとに固定し、環境変数やマニフェストでは設定しません（URL 変更にはソースレビューと再ビルドが必要）。未知・未設定のプロジェクトでは窓口を推測せず、お問い合わせボタンを表示しません。
- プライバシー請求に FAQ 事前読了やマイページへのログインを求めず、受付後は原則同じチャンネルを新しい YouTube の許可（fresh OAuth）で確認します（通常ログインだけでは実行せず、利用できない場合は窓口で個別方法を案内）。

#### お問い合わせ窓口の公開前条件（ソースのマージ条件には含めない）

- マイページ未ログイン・利用制限中でも窓口に到達できることと、Google 未ログイン状態で環境別フォームから試験回答を送信し運営者の受信・返信・依頼者への到達まで実証すること（OAuth 同意画面の support email や既存匿名窓口は自動転用しない）。
- フォーム回答の保管場所・閲覧権限・保持期間・削除方法と担当を確定すること（受付時刻・返信先・対応担当・期限は人的窓口で管理し、対応結果は元の窓口から返し、実データ削除と 7 暦日以内の期限管理を別途実証）。
- [Google 側で連携を解除した利用者への対応](../support-operator.md#google側で連携を解除した利用者への対応) を開発環境で通し確認し、正式手順化と fresh OAuth 不可時の本人確認の最低基準を公開前に判断すること。

確定済みの事項:

- 受付担当は運営者本人（らいど開発）が担う。返信は元の窓口から行い、内部目標は数営業日以内とする（公開約束は受付から7暦日以内）。通知先・保管場所・閲覧権限・保持期間・削除方法の実値は0A初回リリース前に実フォームで確定し、受信・返信の到達を実証する。
- 公開本文は privacy v1 / terms v1 とし、2026-11-17に運営者が正式承認した。施行日は初回本番公開日（両文書同日）とし、公開直前にソースの定数を更新する。

---

## ローカル検証

依存関係は各パッケージのロックファイルとツールチェーン設定を使い、秘密情報や実ユーザーデータは不要です。以下はリポジトリ内の検査であり実 OAuth / Hosting を代替しません。

```sh
# root
python3 .github/scripts/check-doc-references.py
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
```

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

Go 変更の必須検査は [AGENTS.md](../../../AGENTS.md) に従い生成後の意図しない差分がないことを確認し、Firestore 依存の受入検証はルートから [既存 Emulator スクリプト](../../../.github/scripts/run-firestore-integration-tests.sh) を `demo` プロジェクトで実行します。

状態遷移は [ランタイムテスト](../../../mypage/src/runtime.test.ts) と [プライバシーテスト](../../../mypage/src/privacy.test.ts) で検証し、ブラウザ確認はローカル Vite（ポート 18081）を起動して [表示確認の入口](../../../mypage/src/visual.tsx) と [フォント QA](../../../mypage/scripts/font-qa.py) で行います（設定済み公開文面は専用テストと実ブラウザで匿名到達と安全なリンクを確認し、実ユーザー画面は成果物に保存しません）。

---

## DEV Hosting を CI で配備する

ビルド時に既存の React コンポーネントから規約本文を含む静的 HTML を生成します。Firebase Hosting は `/privacy` と `/terms` にその HTML を返すため、ログインや JavaScript の実行なしで本文を読めます（JavaScript が動くブラウザでは Cookie 設定や画面内遷移も引き続き利用可能）。Google OAuth 同意画面に登録済みの `/privacy_policy` は HTTP 302 で `/privacy` へ転送します（反映にはフロントエンド再デプロイが必要）。

DEV ページの更新は基盤 Terraform Apply とは別に `Deploy MyPage Frontend`（[`deploy-mypage-frontend.yml`](../../../.github/workflows/deploy-mypage-frontend.yml)）を統合ブランチ `integration/mypage-canon-20261006` から手動実行します（実行 SHA は選択 ref の `github.sha` を使い手入力しない。PR 実行・`main`・他ブランチ・別所有者・再実行・SHA 不一致は拒否し、失敗時は同じ Run を再実行せず現在状態を確認して新しく実行）。

1. 同じ Run 内でまず MyPage の認証なし CI（`ci-frontend.yml` の `mypage`: audit・cost model・フォント整合性・check・test・build）を通し、続くクラウド認証情報なしの Build ジョブで固定依存検査・check・typecheck・テスト・DEV 公開設定検査・ビルドを実行します。
2. 成果物を別 Apply ジョブへ渡し、ソース SHA・Run ID・30 分の期限・設定・全ファイルの SHA256 を再検証してから専用 WIF と短命トークンで認証します（SA 鍵や Firebase ログイントークンは不使用）。
3. 対象を `test-youtube-study-space` の `test--study-space` だけとし、現在の Hosting の CSP・書き換え等と API の固定オリジン・Ready リビジョンを照合したうえでリリースを切り替えます（自動再試行しない）。
4. 配備後は全ファイルのバイト列・`Cache-Control`・CSP・`/` と `/mypage`・匿名 API の 403・API メタデータ不変を確認し、別の認証なし Browser ジョブで匿名画面・Cookie 設定・同意バナー表示・同意前と拒否後の GA4 通信 0 件を確認します（実 GA4 送信・認証済み E2E・公開条件完了とは区別）。

- GitHub Environment の `mypage-dev-hosting` には公開項目だけの `MYPAGE_HOSTING_PUBLIC_CONFIG_JSON` と専用プロバイダー名 `MYPAGE_HOSTING_WIF_PROVIDER` を Variables に設定し、デプロイ時に DEV の測定 ID・公開オリジン・固定 CSP 許可先（`https://www.googletagmanager.com/gtag/js`、`https://www.google-analytics.com`、`https://region1.google-analytics.com`）を補います（変数に残る旧項目の `VITE_MYPAGE_ANALYTICS_READY` と同意版・`publicPolicy` は検査前に取り除くため、変数から段階的に削除できます）。有効化後は検証用ブラウザでアクセス解析を許可し Network と GA4 Realtime で受信を照合し、拒否へ戻して通信が止まることも確認します。
- 権限は専用の `mypage-hosting` WIF pool/provider と `mypage-hosting-deploy` SA に分離します。Firebase Hosting にはカスタムロールがないため DEV プロジェクトに `roles/firebasehosting.admin` を付与しつつ、サイト 1 個への限定は固定 API/配備仕様と厳格な WIF 受入検査で担保します（加えて `mypage-api` 1 サービスの `roles/run.viewer` と DEV `roles/serviceusage.serviceUsageConsumer` だけを追加）。
- Firebase のリリース切替 API には compare-and-swap がないため実行中は別経路の更新を避け、リリース記録は切替 API 呼び出し前に `unknown` で保存して応答確認後に `confirmed` へ更新します（成否にかかわらず CI 成果物で 14 日間保存、ビルド成果物は 1 日で期限切れ。失敗時は `previousVersion` / `previousRelease` と現在配信を照合して手動復旧判断）。本番経路は公開条件・独立 Environment/権限レビュー後に追加します。

### 作業集計 API の切り替え

- [#1572](https://github.com/kani3camp/youtube-study-space/issues/1572) の反映時は旧画面が新 API を解釈できるため Backend を先にデプロイしてから Hosting を更新し、両方戻す場合は Hosting を先に戻します。
- 新 Backend は旧環境変数 `MYPAGE_HISTORY_COVERAGE` を参照しません。次回の Terraform Plan 前に Cloud Run に設定が残っていないか確認してください（残存時は新構成が削除を計画して現行ガードで停止するため、対象変数だけを外す手順を別途レビューします）。

### 同意版を変える配備

- 同じコミットから Backend（`Deploy MyPage API`）を先に配備し、続けて Frontend（`Deploy MyPage Frontend`）を配備します。画面と API の版が異なる間は、ログイン開始が `POLICY_VERSION_OUTDATED` で拒否されます（誤った版の同意は記録されません）。
- 配備後に実ブラウザで同意してログインを開始し、Google の認証画面へ進むことを確認します。
- Backend は旧環境変数 `MYPAGE_PRIVACY_VERSION` / `MYPAGE_TERMS_VERSION` を参照せず、Terraform もこの 2 変数を Cloud Run に設定しません。既存の Service に残っている場合、次の基盤 Plan は 2 変数だけを外す in-place update を計画し、Plan 契約はこの更新だけを許可します（image・他の環境変数とその順序・Secret 参照・権限・トラフィックの変更や置換が混ざれば停止します。2 変数が消えた後は該当しないため、許可は一度で失効します）。この Apply は新しいリビジョンを作るため、実行中は `Deploy MyPage API` を走らせないでください。
- GitHub Environment の `MYPAGE_RUNTIME_SETTINGS_JSON` に残る旧 `privacy_version` / `terms_version` は読み込み時に除外されるため、編集は不要です。
- 同意版が変わると、旧版で同意したアカウントは次回ログイン時に再同意を求められます（DEV の旧版は `dev-draft-*-30102f3`）。

### 静的配信キャッシュの仕様

同じ公開マニフェストでフロントエンドをビルドし、実 `dist` を専用リリースディレクトリ内の `dist/` へコピーして以下で設定候補を生成します（配備はしません）。

```sh
node mypage/scripts/prepare-hosting.mjs --manifest /path/to/reviewed-public-config.json --dist /path/to/release/dist --output /path/to/release/firebase.json
```

- 出力先は未作成ファイル・親ディレクトリは作成済みとし、Firebase CLI の仕様上、シンボリックリンク解決後も `dist` は出力設定の親ディレクトリ内の独立サブディレクトリである必要があります（マニフェストや非公開ファイルは置かない）。
- 既知のハッシュ付き JS・CSS・フォントだけを 1 年間 `immutable` とし、HTML・API・未知のハッシュパスは `no-store` とします。ビルド元は追跡済みで変更のない Git worktree とし（QA アーカイブ内をビルドルートにすると Tailwind の走査が変わるため不可）、JS だけの更新時は直前のレビュー済み `dist` と CSS・全フォントの名前と SHA256 を機械比較します。
- Hosting Emulator で既知アセット、`/`、`/mypage`、`/api/mypage`、未知の `/assets/missing-12345678.js` の `Cache-Control` と CSP/`nosniff` を確認します（実バックエンドヘッダーやダウンロード量は別記録）。

---

## DEV 専用：MyPage の Firestore 読み取り原価計測

[原価モデルの前提](../cost-notes.md) に従い対象リリースと予算を固定して計測するためのフックです（ソースだけでは実配備や課金確認を証明しません）。

- DEV で明示的に `MYPAGE_READ_METRICS=true` を設定したときだけ、BFF の Cache Miss から生じた成功した read-only transaction ごとに `mypage_snapshot_read` を出力します（Production では無効）。
- 記録内容は `user_lookups`、`seat_queries` / `seats_returned` / `seat_query_errors`（0〜2）、`segment_queries` / `segments_returned` / `segment_query_error`（true/false）、`elapsed_ms` の数値と真偽値のみで、UID・チャンネル ID・氏名・パス・作業名・生エラーなどは含めません。返却 0 件でも対応するエラー数・フラグが `0` / `false` の場合だけ正常な空結果とし、未実行は query 数 `0` で区別します（partial snapshot でトランザクションが成功する場合も失敗指標を残しますが、トランザクション全体の失敗時はコールバック自体が呼ばれません）。
- この数値を Firestore の実課金読み取り数と混同しないでください（0 件クエリ最低課金、index entry read、transaction retry、アクセス制御別経路、AWS Batch 等は含まれません。実請求は Cloud Billing と、小規模データでの [Query Explain](https://firebase.google.com/docs/firestore/query-explain) により複数範囲条件を持つ `work-segments` の index entry read を確認します）。
- 合成テスト対象は User 不存在、履歴 0/10/100/500/1000 件（可能なら 1001 件検知）、同一 ID の通常席/メンバー席、JST 日曜→月曜・月末境界、データ不整合、クエリ失敗と認証拒否とし、失敗や上限時に履歴を 0 として表示しないことも確認します。
- 1 人・1 日あたりのリクエスト回数と毎回の処理量を組み合わせて実測原価を計算します（代表的な 2 時間滞在なら、定期 refresh は旧 60 秒 120 回→新 5 分 24 回が目標値。writer は [削除 inventory の起動経路](../support-delete-inventory.md#起動と再生成経路の調査先) と原記録 writer を照合）。

### 月次履歴の明示移行と復旧

初期リリースは原記録を直接読みます。DEV 専用 CLI による月次集計の切替はリリース手順から外しますが、DEV に残る派生値・マーカーの確認や個人情報削除では原記録と派生値の両方を対象にします。

---

## 確認と承認の順序

| 確認段階 | 完了を示す根拠 | 未完時の扱い |
| --- | --- | --- |
| MyPage Ready Gate | [infra 正本 Phase 6](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc) の dev/prod 基盤・ownership・Firebase 境界・keyless CI | リソース本実装/プロビジョニングへ進まない |
| Phase 0A 準備 | exact commit/CI、設定事前検査、公開文面と API 前同意、対象/ロールバック/実試験計画 | ローカル準備のみ継続（ターゲット値を推測しない） |
| 0A 実施承認 | 対象 project/domain/revision、操作一覧、本人 test accounts、infra/security 確認記録、秘密供給方式 | 実環境実行/公開/追加 IAM を行わない |
| 0B 受入 | 0A 実経路と Canon S01/S04 合格、MON（monitor 認証・OBS 永続化・失敗通知・max-seat 回帰）、COST（実課金境界・Budget 実着信・異常監視）、D01 実装証拠、D02 両取消経路の規約適合性確認、D03 実値、人的 Privacy 窓口、7 暦日削除/cleanup、実 metadata/log/backup 棚卸し | 実ユーザー受入を停止 |
| Phase 1 一般公開 | OAuth 審査完了、0B 実アカウント/端末 QA、Bot/外部導線、ロールバック、監視 | 日付や CI 成功だけで告知しない |

### 人的Privacyと削除の運用

初期公開のプライバシー請求はログイン不要の有人窓口と非公開削除運用を前提とします（アプリ内受付・返信 UI は Phase 1 対象外として撤去済み。セッションなしの場合も同じ有人窓口で扱い、新しい Bearer 認証情報は導入しません）。

- D03 で返信できるログイン不要の窓口・担当・受付時刻と期限管理（受付から 7 暦日、証拠再発行・ACK 消失・再試行で延長しない）・実公開文面を確定し、匿名一般意見窓口や OAuth user support email を自動転用しません。受付後は依頼に結び付けた新たな OAuth 認証で本人確認し、通常ログインだけでは代用しません。
- 非公開 CLI はオーナー管理の ADC/GCP IAM へ接続し、[削除対象一覧](../support-delete-inventory.md) で実ランタイム・旧書き込み処理・Auth/BQ/キャッシュ/バックアップ/エクスポート/ベンダー/ログを確認して横断削除を行います（部分失敗は保留のまま再開）。
- D02 では外部 Google 取消検知のためのリフレッシュトークン保存・定期取得を採用せず、アプリ内と Google 設定画面の両取消経路および保持済み API データの扱いに関する検証・規約適合性確認が終わるまで公開を保留します（OAuth スコープは `youtube.readonly` のみ、コールバックは `/api/auth/youtube/callback` 固定）。
- Canon S03 のチャンネル譲渡は利用規約で扱い、申告時は [手動ブロック・セッション失効](../service-access-control.md) で対応します。なお、通常セッション失効で最大約 1 時間の残存を許容することは削除時の全書き込み・キャッシュ停止（D01）の代替になりません。

---

## 証跡と停止条件

[リリース記録例](release-record.example.json) に、フロントエンドコミット・Hosting リリース、バックエンドイメージのダイジェスト・リビジョン、`rewrite`/`pinTag`、スキーマ・同意バージョン、試験範囲と参照、所有者承認を一組で記録します（非公開の対象情報・個人データ・認証情報は公開 PR に載せません）。

認証・IDOR の問題、誤チャンネル、`code`/`state` 漏えい、同意前 GA4 送信、旧データ復活、保持・削除期限違反、ロールバックの互換性不足があれば直ちに停止します。部分失敗は `unknown`/`pending` として記録し再実行で失敗を消さず、セキュリティスキャン結果と CI Gate を区別します（解析未開始を合格とみなしません）。
