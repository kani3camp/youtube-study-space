# MyPage 配備・公開準備手順

この文書は、MyPage の設定事前検査・CI 配備・公開前の確認条件を案内します。

> [!IMPORTANT]
> 本書に記載された設定検査やローカルの合成テストの成功は、実経路での合格や一般公開の承認を意味しません。
> 実際の配備・検証結果は対象リリースの Issue / PR / Actions に記録し、全体の統合状況は [PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)、公開前の未完項目は [Issue #1133](https://github.com/kani3camp/youtube-study-space/issues/1133) で追跡します。

| 関連資料 | 内容 |
| --- | --- |
| [MyPage 実装仕様](../README.md) | 認証・集計・キャッシュ・Hosting などの仕様 |
| [OpenAPI 定義](../openapi.yaml) | API エンドポイント仕様 |
| [実環境 E2E 手順](e2e.md) | 受入試験・ロールバック・緊急遮断の手順 |
| [メンテナンスモード v1](../maintenance-mode.md) | 計画停止の初期化・開始・解除手順（通常リリースとは独立して手動操作） |
| Notion Current Canon | [04 認証](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd) / [05 Privacy](https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf) / [06 Security](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634) / [09 判断記録](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f) |

---

## 構成と配備の入口

開発（DEV）・本番（PROD）ともに、Firebase Hosting（React/Vite）、`/api/**` の書き換え転送、Cloud Run Go BFF、Firebase Authentication、Firestore で構成します。

- **Cloudflare の用途**: ドメイン登録と DNS のみに使用します。初期構成に Managed WAF・Cloud Armor・専用外部 LB・Cloud Run Web は含みません。
- **直接アクセス境界の確認**: Cloud Run 標準の `run.app` URL にも直接到達できるため、Hosting 経由と直接アクセスの両方で Auth・App Check・ServiceAccessControl（SAC）・Origin・UID 制限・Canon S01/S04 を検証します。
- **公開ドメイン**: `https://moku.work/`（登録の Active 状態、DNS/TLS、OAuth コールバック、本番配備は対象環境を再読み取りして確認します）。

| 対象 | 手順・管理境界 |
| --- | --- |
| 基盤・鍵を使わない認証・環境別 State | [MyPage 基盤 Terraform](../../../infra/mypage-platform/README.md) |
| OAuth トランザクションの TTL | [OAuth TTL 専用 Terraform](../../../infra/mypage-oauth-ttl/README.md)（アプリ側の 10 分失効と非同期 TTL 削除を区別） |
| バックエンドイメージ更新 | [バックエンド CI 手順](backend-ci.md)（検査 → イメージビルド → digest 検証 → Cloud Run への直接配備。通常リリースで Terraform は起動しません） |
| フロントエンド | 本ページの「[DEV Hosting を CI で配備する](#dev-hosting-を-ci-で配備する)」と「[静的配信キャッシュの仕様](#静的配信キャッシュの仕様)」 |
| メタデータ保持・監視 | [削除処理と独立監視](metadata-monitoring.md)（ソース実装と実スケジューラー・通知先の接続確認を区別） |
| 横断削除 | [削除手順](../support-delete.md)・[調査対象一覧](../support-delete-inventory.md)（CLI 実装やローカル検証だけでは実削除運用の完了とみなしません） |

---

## 公開設定

### GA4 の環境別セットアップ

MyPage の GA4 には、各 Firebase プロジェクトに接続済みの `mypage` 用プロパティと、`my-page` ストリームを使用します。DEV 環境では同じ GA4 プロパティ内に `youtube-monitor` ストリームもあるため、プロパティ単位の設定を変更する際は両ストリームへの影響に注意してください。

- **測定 ID**: DEV `G-E31NHW939V` / PROD `G-ZJ1VPVT2L2`
  - ストリーム URL は承認済みの環境別公開オリジンに揃え、開発アクセスが本番集計へ混入しないようにします。
  - ID の採用は確定していますが、環境別ビルド設定への反映・管理画面の受入・実通信と Realtime の検証が終わるまでは、実送信の有効化完了とはみなしません。
- **GA4 コンソールの必須設定**:
  - 広告連携・Google Signals・広告パーソナライズを使用しない。
  - データ保持期間を **14 か月** に設定し、「新しいアクティビティで保持期間をリセット」を **OFF** にする。
  - SPA 側で手動ページビューを送るため、Enhanced Measurement の「履歴変更に基づくページビュー」など重複・余分な自動送信を無効にする。
- **ビルド設定と CSP**:
  - `VITE_MYPAGE_GA4_ID=<その環境の測定 ID>` と `VITE_MYPAGE_PUBLIC_ORIGIN=<その環境の HTTPS origin>` を環境別のリリース設定に必ず含めます（測定 ID は公開値であり、シークレット扱いしません）。
  - 公開マニフェストの整合性検査と、Hosting CSP 許可先の別途承認が必要です。
- **同意バナーの扱い**:
  - GA4 未接続時は初回同意バナーを表示しません。既存の v1 同意設定は GA4 接続前に取得した選択のため再利用せず、v2 へ更新します。
  - GA4 を有効化した最初のアクセスで、サイト改善のためのアクセス解析について案内し、利用者に明示的な同意または拒否を選んでもらいます。
- **DEV からの実ブラウザ検証**:
  - 同意前・拒否中の GA4 通信が 0 件であること、許可後の実イベントと URL 秘匿、拒否切り替え後の通信停止と Cookie、Realtime 表示、CSP、外部送信表示を確認します（ローカル合成テストは実ベンダー動作を証明せず、DEV 結果を本番確認の代わりにはしません）。

---

## 外部接続なしの設定事前検査

公開設定は [firebase.ts](../../../mypage/src/firebase.ts)、サーバー設定は [main.go](../../../system/cmd/mypage-server/main.go) の仕様に揃えます。設定値は対象プロジェクト・アプリの管理画面や承認済み一覧から取得し、サンプル値をそのまま実環境へ流用しないでください。

```sh
# リポジトリルートで実行: 公開専用 JSON のみを検査（SDK / ADC / 外部通信なし）
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json

# 非公開マニフェストを標準入力で渡す例（サーバー環境全体は export しない）
node mypage/scripts/check-release-config.mjs --stdin < /path/to/private-public-config.json

# system/ で実行: 現在のプロセス環境変数を既存バリデータで検査（ADC / listen / 通信なし）
cd system
go run ./cmd/mypage-server --check-config
```

- **バックエンド検査の境界**: 既存の安全なプロセス環境からサーバー専用シークレットを読みますが、コマンド引数・マニフェスト・Vite へは移しません。出力は固定の `status` / `field` / `code` のみです。
- **公開設定チェッカーの仕様**: 64 KiB 以内の明示 JSON を読み込み、秘密値・デバッグ値・未知フィールドや、プロジェクト・アプリ・オリジン・同意バージョンの不一致を拒否します。
  - 最上位キー: `schemaVersion: 1`、`deployment`、`frontend`、および任意の `backend`、`hosting`、`publicPolicy`
  - `deployment` / `backend`: `environment`、`projectID`、`projectNumber`、`webAppID`、`publicOrigin`、`privacyVersion`、`termsVersion` の 7 項目
  - `frontend`: [public-config.example.json](public-config.example.json) と [release-config.ts](../../../mypage/src/release-config.ts) を参照
  - `publicPolicy`: `VITE_PUBLIC_CONTACT_URL`、`VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE`、`VITE_PUBLIC_TERMS_EFFECTIVE_DATE`、`VITE_PUBLIC_JURISDICTION`、および任意の `VITE_PUBLIC_OPERATOR_NAME`（既知の Firebase プロジェクトでは問い合わせ先キーを設定しません）
  - `hosting`: [hosting.ts](../../../mypage/src/hosting.ts) の `HostingInputs` 形式を使用（任意入力を省略した場合は `not-supplied` として未検証のまま残します）

> [!NOTE]
> `configurationValid=true` や `configuration=valid` は設定形式の整合だけを示すものであり、外部登録・鍵の真正性・権限・公開本文の承認や E2E 完了を意味しません。`MYPAGE_INFRASTRUCTURE_READY=true` も起動に必要な宣言であり、確認完了の証明には使えません。

### 設定の受け渡し一覧

Hosting の画像接続先一覧（`img-src` 専用）には、チャンネル画像用の `https://yt3.ggpht.com` と `https://yt3.googleusercontent.com` の両方を明示します（reCAPTCHA 画像のみの指定は事前検査で拒否されます）。将来、別の画像オリジンが返るようになった場合は実配信元を確認して一覧を再レビューします。

| 用途 | 項目 | 受け渡し・確認ルール |
| --- | --- | --- |
| 公開ブラウザ設定 | `VITE_FIREBASE_API_KEY` / `AUTH_DOMAIN` / `PROJECT_ID` / `APP_ID`、`VITE_APP_CHECK_SITE_KEY` | 公開 Web App 設定（secret / admin key / debug token は不可）。Project 番号・App・登録ドメインを別途照合 |
| 同意バージョン | ブラウザ `VITE_PRIVACY_POLICY_VERSION` / `VITE_TERMS_VERSION`<br>サーバー `MYPAGE_PRIVACY_VERSION` / `MYPAGE_TERMS_VERSION` | 同一リリースの組で一致させ、実際に承認された本文の版を記録 |
| 公開オリジン | `MYPAGE_PUBLIC_ORIGIN`<br>解析用 `VITE_MYPAGE_PUBLIC_ORIGIN` | 環境ごとに固定した HTTPS オリジン（リクエストヘッダーから推測しない） |
| サーバー識別情報 | `MYPAGE_ENVIRONMENT` / `REGION` / `PROJECT_NUMBER` / `WEB_APP_ID` / `SIGNER_EMAIL`、`GOOGLE_CLOUD_PROJECT` | `development` = `asia-southeast2`、`production` = `asia-northeast2`。署名者は選択プロジェクトのもの |
| サーバー秘密情報 | `MYPAGE_OAUTH_CLIENT_ID` / `CLIENT_SECRET`、任意の `MYPAGE_YOUTUBE_API_KEY` | 承認済みのシークレット供給境界から渡し、リポジトリ・チャット・公開成果物・Vite に入れない |
| GA4（リリース必須） | `VITE_MYPAGE_GA4_ID`、`VITE_MYPAGE_PUBLIC_ORIGIN` | 環境別の測定 ID と公開オリジン（送信には利用者の同意が必要） |
| 公開案内 | 任意の `VITE_PUBLIC_OPERATOR_NAME`、`VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE` / `TERMS_EFFECTIVE_DATE` / `JURISDICTION` | 公開可能な値のみを使用（実名・自宅住所を必須にしない）。文面は草案のまま保持 |
| お問い合わせ先 | [`public-policy-config.ts`](../../../mypage/src/public-policy-config.ts) の `contactForms` | 一般・プライバシーの窓口で共通フォームを使用。`VITE_FIREBASE_PROJECT_ID` が `test-youtube-study-space` または `youtube-study-space` の場合は環境別固定 URL を優先し、未知・未設定プロジェクトでのみ `VITE_PUBLIC_CONTACT_URL` を参照 |
| ライブ・メンテナンス告知 | [`youtube-channel.ts`](../../../mypage/src/youtube-channel.ts)（キーは `VITE_FIREBASE_PROJECT_ID`） | DEV は `UChBj4xZePvWyGmTKct5FBcg`、PROD は `UCXuD2XmPTdpVy7zmwbFVZWg` の `/live`・`/posts` を参照。未知・未設定プロジェクトでは両導線を非表示にし、変更にはソースレビューと再ビルドを必須化 |
| Bot 設定案 | `MYPAGE_PUBLIC_ENTRY_URL`（runtime 未接続） | 承認済みの公開入口のみ（ユーザー固有のクエリ・トークン不可）。0A/0B は未設定、Phase 1 承認後に設定 |

### お問い合わせ窓口の設定と受入

`/contact` では、使い方・不具合・ご意見の「一般窓口」と、Google 連携解除後の連絡やデータ削除・開示・訂正・全ログイン解除などの「プライバシー窓口」を案内します。受付と返信は人的窓口で行い、バックエンドの本人確認・運用には [人的 Privacy 運用 CLI](../support-operator.md) を利用します。

- **問い合わせ先の決定ルール**:
  - 既知の Firebase プロジェクトでは、ソースコードの `contactForms` を正本とし、環境変数の値にかかわらず固定 URL を表示します（URL 変更時はソースレビューと再ビルドが必要）。
  - 未知・未設定のプロジェクトでは、公開マニフェストの `publicPolicy.VITE_PUBLIC_CONTACT_URL` を参照します（`mailto:` または HTTPS URL のみ許可し、認証情報やクエリは含めません。未設定・不正値時はボタン非表示）。
- **プライバシー請求の要件**: 請求に FAQ の事前読了やマイページへのログインを条件にしません。受付後は原則として対象チャンネルを新しい YouTube OAuth（fresh OAuth）で確認します（通常ログインだけでは実行しません）。連携解除済みなどで利用できない場合は窓口で個別確認の方法を案内します。

#### お問い合わせ窓口の公開前 Gate（ソースのマージ条件には含めない）

- マイページ未ログイン・利用制限中の状態および Google 未ログイン状態で環境別フォームに到達し、試験回答を送信して運営者の受信・返信・依頼者への到達まで実証する（OAuth 同意画面の support email や既存匿名窓口は自動転用しない）。
- フォーム回答の保管場所・閲覧権限・保持期間・削除方法と担当を確定する。受付時刻・返信先・担当・期限（受付から 7 暦日以内）は人的窓口で管理し、対応結果は元の窓口から返信する。
- [Google 側で連携を解除した利用者への対応](../support-operator.md#google側で連携を解除した利用者への対応)（暫定手順）を DEV 環境で通し確認し、正式手順化と fresh OAuth 不可時の本人確認の最低基準を公開前に決定する。

---

## ローカル検証

依存関係は各パッケージのロックファイルとツールチェーン設定を使用します（シークレットや実ユーザーデータは不要です）。

```sh
# リポジトリルート: 公開設定・ドキュメントリンクの検査
python3 .github/scripts/check-doc-references.py
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
```

```sh
# system/ で実行（詳細は AGENTS.md を参照）
cd system
go test -shuffle=on ./...
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
```

```sh
# mypage/ で実行
cd mypage
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

- Firestore に依存する統合テストはルートから [run-firestore-integration-tests.sh](../../../.github/scripts/run-firestore-integration-tests.sh) を実行します（`demo` プロジェクトを使用し、実認証情報は渡しません）。
- 状態遷移は [runtime.test.ts](../../../mypage/src/runtime.test.ts) と [privacy.test.ts](../../../mypage/src/privacy.test.ts) で検証します。
- ブラウザ上の表示・フォント・公開文面はローカル Vite（ポート `18081`）を起動し、[visual.tsx](../../../mypage/src/visual.tsx) や [font-qa.py](../../../mypage/scripts/font-qa.py) で確認します（実ユーザー画面は成果物に保存しません）。

---

## DEV Hosting を CI で配備する

ビルド時に既存の React コンポーネントから規約本文を含む静的 HTML を生成します。Firebase Hosting は `/privacy` と `/terms` にその HTML を返すため、ログインや JavaScript の実行なしでも本文を読めます（JavaScript が有効なブラウザでは Cookie 設定や画面内遷移も引き続き利用可能）。Google OAuth 同意画面に登録済みの `/privacy_policy` は HTTP `302` で `/privacy` へ転送します。

DEV ページの更新は、基盤の Terraform Apply とは独立して `Deploy MyPage Frontend`（[`deploy-mypage-frontend.yml`](../../../.github/workflows/deploy-mypage-frontend.yml)）を統合ブランチ（`integration/mypage-canon-20261006`）から手動実行します。失敗時に同じ Run を再実行（Re-run）せず、現在状態を確認したうえで新しく実行してください。

1. **事前検査・ビルド**: 同じ Run 内でまず `ci-frontend.yml` の `mypage`（`pnpm audit`・コストモデル・フォント整合性・check・test・build）を通し、続いてクラウド認証情報なしの Build ジョブで DEV 公開設定検査とビルドを実行します。
2. **成果物検証と認証**: Build ジョブの成果物を Apply ジョブへ渡し、ソース SHA・Run ID・30 分の有効期限・設定・全ファイルの SHA-256 を再検証したうえで、専用 WIF と短命トークンで認証します（SA 鍵や Firebase ログイントークンは不使用）。
3. **Hosting リリース切替**: 対象を `test-youtube-study-space` の `test--study-space` に限定し、現在の Hosting CSP・書き換え設定と API の同意バージョン・固定オリジン・Ready リビジョンを照合したうえでリリースを切り替えます（自動再試行はしません）。
4. **配備後検証**: 全ファイルのバイト列・`Cache-Control`・CSP、`/` と `/mypage`、匿名 API の `403`、API メタデータの不変を確認します。別の認証情報なし Browser ジョブで匿名画面・Cookie 同意バナー表示・同意前および拒否後の GA4 通信が 0 件であることを確認します。

> [!NOTE]
> - GitHub Environment `mypage-dev-hosting` の Variables には、公開項目のみの `MYPAGE_HOSTING_PUBLIC_CONFIG_JSON` と専用プロバイダー名 `MYPAGE_HOSTING_WIF_PROVIDER` を設定します（サーバー環境やシークレットは渡しません）。デプロイ時に DEV の測定 ID、公開オリジン、固定 CSP 許可先（`https://www.googletagmanager.com/gtag/js`、`https://www.google-analytics.com`、`https://region1.google-analytics.com`）を補います。
> - 有効化後は、検証用ブラウザでアクセス解析を許可して Network と GA4 Realtime で受信を確認し、拒否へ戻して通信が止まることも確認します。
> - IAM 権限は専用 WIF（`mypage-hosting`）と SA（`mypage-hosting-deploy`）に分離します。Firebase Hosting にはカスタムロールがないため DEV プロジェクトに `roles/firebasehosting.admin` を付与しつつ、対象サイト 1 件への限定は配備仕様と厳格な WIF 受入検査で担保します（加えて `mypage-api` の `roles/run.viewer` と DEV `roles/serviceusage.serviceUsageConsumer` のみを付与）。
> - Firebase のリリース切替 API には compare-and-swap がないため、実行中は他経路からの Hosting 更新を避けます。リリース記録は切替前に `unknown` で保存して応答確認後に `confirmed` へ更新し、CI 成果物として 14 日間保存します（ビルド成果物は 1 日）。失敗時は `previousVersion` / `previousRelease` と現在配信を照合して明示的に復旧判断します。

### 作業集計 API の切り替え

- [Issue #1572](https://github.com/kani3camp/youtube-study-space/issues/1572) の反映時は、旧フロントエンドが新 API を解釈できるため **Backend を先にデプロイしてから Hosting を更新** し、戻す場合は Hosting を先に戻します。
- 新 Backend は旧環境変数 `MYPAGE_HISTORY_COVERAGE` を参照しません。次回の Terraform Plan 前に Cloud Run 上に残存していないか確認してください（残存している場合は新 Terraform 構成が削除を計画するため、現行の構成変更ガードで停止します）。

### 静的配信キャッシュの仕様

同じ公開マニフェストでビルドした `dist/` を使い、`prepare-hosting.mjs` でリリースの候補設定を生成できます（配備は行いません）。

```sh
node mypage/scripts/prepare-hosting.mjs \
  --manifest /path/to/reviewed-public-config.json \
  --dist /path/to/release/dist \
  --output /path/to/release/firebase.json
```

- **ビルドルートと資産の保護**: ビルド元は追跡済みで変更のない Git worktree とし、マニフェストや非公開運用ファイルを `dist/` へ入れません。Firebase CLI の制約により、シンボリックリンク解決後も `dist` は出力設定の親ディレクトリ内の独立サブディレクトリである必要があります。
- **キャッシュ方針**: 実ビルド済みのハッシュ付き JS・CSS・フォントだけを `public, max-age=31536000, immutable` とし、HTML・API・未知のハッシュパスは `no-store` とします。JS のみの更新時は直前のレビュー済み `dist` と CSS・全フォントの名前と SHA-256 を機械比較し、意図しない変化があれば配備前に停止します。
- **検証方法**: Hosting Emulator で既知アセット、`/`、`/mypage`、`/api/mypage`、未知パスの `Cache-Control` / CSP / `nosniff` を確認します。

---

## DEV 専用：MyPage の Firestore 読み取り原価計測

[原価モデルの前提](../cost-notes.md)に沿って計測するための観測フックです（ソース実装だけでは実 DEV での有効化や課金確認を完了したことにはなりません）。

- **有効化条件**: DEV 環境で明示的に `MYPAGE_READ_METRICS=true` を設定した場合のみ、BFF キャッシュミスで実行された **成功した読み取り専用トランザクション** ごとに `mypage_snapshot_read` ログを出力します（本番環境では無効）。
- **出力項目（秘匿値なし）**: `user_lookups`、`seat_queries` / `seats_returned` / `seat_query_errors`（0〜2）、`segment_queries` / `segments_returned` / `segment_query_error`（true/false）、`elapsed_ms` の数値・真偽値のみを記録します。UID・チャンネル ID・氏名・作業名・トークン・生エラーは含めません。
- **未実行と 0 件の区別**: `seats_returned == 0` かつ `seat_query_errors == 0` などの場合のみ正常な空結果とし、未実行時は query 数 `0` で区別します。partial snapshot でトランザクション自体が成功した場合も失敗指標は残りますが、トランザクション全体の失敗時は観測コールバック自体が呼ばれません。
- **実課金との違い**: この指標には 0 件クエリの最低課金、インデックスエントリーの読み取り、トランザクション再試行、SAC 確認、AWS Batch などの利用は含まれません。実請求は Cloud Billing と [Query Explain](https://firebase.google.com/docs/firestore/query-explain)（`work-segments` の複数範囲条件におけるインデックス読み取り課金を含む）を小規模データで確認します。
- **定期更新の目安**: 2 時間の滞在を例にすると、定期更新は従来の 60 秒ごと（120 回）から 5 分ごと（24 回）が目標値となります。

### 月次履歴の扱い

初期リリース（Phase 0A）では原記録を直接読みます。DEV 専用 CLI による月次集計切替はリリース手順から外しますが、DEV に残存する派生値・マーカーの確認や個人情報削除時は原記録と派生値の両方を対象にします。

---

## 確認と承認の順序

| 確認段階 | 完了を示す根拠 | 未完了時の扱い |
| --- | --- | --- |
| **MyPage Ready Gate** | [infra 正本 Phase 6](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc) の dev/prod 基盤・ownership・Firebase 境界・keyless CI | リソース本実装・プロビジョニングへ進まない |
| **Phase 0A 準備** | exact commit / CI、設定事前検査、公開文面と API 前同意、対象・ロールバック・実試験計画 | ローカル準備のみ継続（ターゲット値は推測しない） |
| **Phase 0A 実施承認** | 対象 project / domain / revision、操作一覧、検証アカウント、infra/security 確認記録、シークレット供給方式 | 実環境実行・公開・追加 IAM を行わない |
| **Phase 0B 受入** | 0A 実経路と Canon S01/S04 の合格、MON（monitor 認証・OBS 永続化・通知・max-seat 回帰）、COST（実課金・Budget・異常監視）、D01/D02/D03 確定、人的 Privacy 窓口・7 暦日削除、実 metadata / log / backup 棚卸し | 実ユーザー受入を停止 |
| **Phase 1 一般公開** | OAuth 審査完了、0B 実アカウント・端末 QA、Bot/外部導線、ロールバック、監視 | 日付や CI 成功だけで一般告知しない |

### 人的Privacyと削除の運用

初期公開のプライバシー請求は、ログイン不要の有人窓口と非公開の削除運用を前提とします（アプリ内受付・返信 UI は Phase 1 対象外として撤去済み）。

- **窓口と本人確認の固定**: 返信可能でログイン不要の窓口・担当・受付時刻と期限管理（受付から **7 暦日**、再発行や再試行で延長しない）・実公開文面を確定します（一般匿名窓口や OAuth 同意画面の user support email は自動転用しません）。受付後は依頼に紐づく新しい YouTube OAuth（fresh OAuth）で本人確認を行い、通常ログインだけでは代用しません。
- **非公開 CLI による削除**: 所有者が管理する ADC / GCP IAM に接続し、[削除対象一覧](../support-delete-inventory.md)をもとに実ランタイム・旧書き込み処理・Auth / BigQuery / キャッシュ / バックアップ / エクスポート / ベンダー / ログを確認して横断削除を行います。部分失敗時は保護を解除せず保留のまま再開します。
- **Google 連携解除への方針（D02）**: 外部での連携解除を検知するためだけにリフレッシュトークンを保存・定期取得する方式は採用しません。アプリ内・Google 設定画面の双方の取消経路に関する規約適合性確認が終わるまで一般公開を保留します。
- **チャンネル譲渡（Canon S03）**: 利用規約で扱い、申告時は [ServiceAccessControl](../service-access-control.md) による手動ブロックとセッション失効で対応します。
- **セッション失効との違い**: 通常の Firebase セッション失効で最大約 1 時間の残存を許容する仕組みは、削除時の全書き込み・キャッシュ停止（D01）の代替にはなりません。

---

## 証跡と停止条件

- **リリース記録**: [release-record.example.json](release-record.example.json) に、フロントエンド SHA・Hosting リリース、バックエンド digest・リビジョン、rewrite 先・`pinTag`、スキーマ・同意バージョン、試験範囲、所有者承認を一組で記録します（非公開の対象情報・個人データ・認証情報は公開 PR に載せません）。
- **停止条件**: 認証・IDOR の不具合、誤チャンネル判定、`code` / `state` 漏えい、同意前 GA4 送信、旧データ復活、保持・削除期限違反、ロールバック互換性不足が見つかった場合は直ちに停止します。部分失敗は `unknown` / `pending` として記録し、再実行で失敗履歴を上書き消去しません。
