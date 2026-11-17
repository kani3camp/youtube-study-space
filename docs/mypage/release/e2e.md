# MyPage 実環境 E2E とロールバック手順

対象リリースの実環境（Google / Firebase / Hosting）経路で受入試験を行うための手順書です。[配備・公開準備手順](README.md) とあわせて使用し、結果はリリース記録と対応する Issue / PR / Actions に残します（ローカルの合成検証だけでは実環境の合格とみなしません）。

---

## Phase 0B 公開前受入（開始前の前提条件）

正式構成は Firebase Hosting `/api/**` → Cloud Run Go API + Firebase Auth / Firestore です（Managed WAF・外部 LB なし）。詳細は [Current Canon 06](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634)、[公開受入 #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)、[monitor auth #1459](https://github.com/kani3camp/youtube-study-space/issues/1459) を参照してください。

Phase 0A の運営者限定試験と並行して準備できますが、以下の項目が未確認のまま **0B を開始しないでください**。

| Gate | 実経路での確認内容 | 現状 |
| --- | --- | --- |
| **MON-01 Rules** | 匿名・通常 MyPage ユーザーから `seats` / `member-seats` / `menu` / `work-name-trend` / `config/constants` の読み取りが拒否されること。有効な許可リスト（`_monitor-readers/{uid}`）の認証済み UID のみ（custom claim 不要）で必要な読み取りが成功し、全クライアントの書き込みが拒否されること | ソース・Emulator 契約あり（DEV / PROD 実環境未実証） |
| **MON-02 OBS** | 実 OBS / CEF 上で、ブラウザソースの Interact ログイン、`browserLocalPersistence`、シーン切替、OBS・PC 再起動、24 時間相当稼働、トークン更新・失効・通知・再ログインを確認 | 未実証 |
| **MON-03 cutover** | DEV / PROD で Rules バージョン・モニター購読・seat-count API 回帰・バックアウト手順を整合させること（認証不能時に匿名 read へ無断ロールバックしない） | 未実証 |
| **COST-01 limits** | Cloud Run の DEV（min 0 / max 1）・PROD（min 0 / max 3）を実環境で確認。timeout / body / query 上限と UID 制限・S01 を Hosting 経由と `run.app` 直アクセスの両方で検証 | 一部ソースあり（実経路要確認） |
| **COST-02 notify** | Billing Budget 通知の着信と、Cloud Monitoring での Run / Firestore reads / 429 / 5xx / instance / YouTube quota の異常通知を確認 | 未実証 |
| **COST-03 cap** | Billing alert は上限停止（hard cap）ではない点を踏まえ、Cloud Run Spend Cap の対象範囲や他サービスへの影響を検証したうえで採否を判断（Firestore には独立した抑止策が必要） | 採否未決 |

※上記を完了するための変更はコード・Rules・クラウド資源ごとに個別レビューし、未承認の本番デプロイや実 OAuth 操作はこの手順の追記だけで実施しません。

---

## 試験記録のルール

- [リリース記録テンプレート](release-record.example.json) に、環境・プロジェクト・ドメイン、フロントエンド SHA・Hosting リリース、バックエンド digest・Cloud Run リビジョン、書き換え先・`pinTag=false`、スキーマ・同意バージョン、承認参照、試験範囲を記録します。
- 秘密情報・個人データの保護: シークレット・トークン・OAuth `code`/`state`・`supportChallenge` / `proofRef`・チャンネル UID・作業内容は公開記録や CI ログに保存しません。生の HAR ファイルも共有・公開せず、非公開の確認資料を秘匿処理したうえで結果・時刻に紐付けます。
- テストアカウントとデータ保護: 運営者が明示したテストアカウントのみを使用します。実データを変更する操作（ログアウト・失効・削除）は操作と対象を別途承認します。未登録や欠損状態を作るために実 DB を直接破損させないでください（実アカウントで用意できない状態は保留とし、ローカル代替の検証範囲を記録します）。

---

## 実経路の受入マトリックス

| ID | 試験内容 | 合格条件 | ローカル事前検証の根拠 |
| --- | --- | --- | --- |
| **P01** | 匿名で `/`・`/privacy`・`/terms`・`/contact` へアクセスと API 前同意 | ログイン不要で公開パスに到達でき、窓口・施行日・管轄・本文の版が承認済み設定と一致 | public policy tests / privacy-qa |
| **A01** | 同意 → start → Google → callback → チャンネル確認 → session complete → MyPage | `youtube.readonly` のみ要求。確認前にトークン発行せず、UID は確認チャンネル、登録済み記録のみ表示 | server integration / runtime-qa |
| **A02** | scope 拒否 / callback 失敗 / 確認キャンセル、期限切れ / リプレイ | トークン永続保存なし。適切な説明と再開始導線があり、誤発行や遅延復活がない | provider / auth / HTTP tests |
| **A03** | 通常アカウント / Brand Account / 複数チャンネル（意図とデフォルトが異なる場合） | 誤チャンネルを確定しない。再試行手順と Google 側の選択挙動を記録 | 実 Google 試験（代替不可） |
| **A04** | Hosting 経由の `__session` / callback（Chrome / Safari / モバイル） | `HttpOnly` / `Secure` / `Lax` / `Path=/`、Hosting 転送、固定 Host、callback 完了 | origin / cookie tests（実 Host は別確認） |
| **A05** | App Check 欠落 / 別アプリ / 失効、Auth 欠落 / 旧 Google provider / 別プロジェクト | データ読み取りやキャッシュより先に拒否。App Check デバッグトークン不使用 | signed synthetic RSA / Firebase boundary tests |
| **A06** | UID 差替え / 余分なクエリ / 別 Origin / 重複 Cookie、並行 callback / confirm | IDOR・Origin を拒否し、claim / consume / mint は 1 回のみ。生の内部エラー非公開 | HTTP origin / auth atomic tests |
| **L01 / Canon S04** | Hosting / API / セキュリティログの callback / query 記録 | `code` / `state` / Cookie / token / support query が不要に記録・通常表示されないこと（全層を確認） | ローカル非出力だけでは証明不可 |
| **Canon S01** | DEV Hosting → Cloud Run で UID 単位の 429 と未認証リクエストの挙動 | UID 制限の 429 は正の `Retry-After` を返し待機後に回復、他 UID へ波及しない。未認証反復で 429 を返さない | プロセス内テストだけでは証明不可 |
| **D01** | 登録済み / 未登録 / 正常 0 分 / 取得失敗 / データ不整合 / 部分障害 | 取得不能と 0 分・未登録を明確に分離し、snapshot / `asOf` と 1,000 件制限が正しく機能 | aggregation / BFF fixtures |
| **D02** | JST 日跨ぎ・退室 / 休憩移動と同時の読み取り | 二重カウントなし。書き込み原子性、キャッシュ失効、`generatedAt` が正常 | repository / workspaceapp / mypage Emulator |
| **M01** | メタデータ正常更新 / 上流失敗 / チャンネル不在 / 30 日境界 | 公開メタデータのみガード付きで更新・消去され、古い表示が復活しない | metadata mock / Emulator |
| **M02** | 定期削除・出力先到達・ハートビート / 失敗アラート | 29 日対象・30 日超保持なし、成功走査間隔 23h 以内・ジョブ 10 分以内、アラート到達 | cleanup / health tests（実結線は別途） |
| **SUP01** | 人的窓口受付 → 依頼専用 fresh support proof（delete / revoke / disclosure）、誤チャンネル / 誤目的 / replay | request / 環境 / 目的 / チャンネル束縛、参照のみ返却、通常ログインへ流用不可（アプリ内受付 UI 不要） | support auth / runtime / Emulator はソース証拠のみ |
| **SUP02** | セッションなし / 本人確認不能 / 期限間近の人的受付 | 返信可能な公開窓口で個別対応し、受付から 7 暦日を延長しない（新 Bearer 認証は不使用） | 窓口・担当・実配信は未確定 |
| **SUP03** | D01 選択後の横断削除 / 再実行 / 復元 / 遅延 writer / キャッシュ | 承認対象のみ削除し、全ストアと backup / export / log を確認。旧データ復活なしで完了通知 | 実アダプター・実棚卸し待ち（dry-run は実行証拠でない） |
| **C01** | GA4（unset / deny / grant / withdraw / 他タブ、callback / support query） | 同意前リクエスト 0 件、過去イベント再送 0 件、秘匿値 0 件、拒否でもログイン可能 | GA4 / privacy / runtime tests（実管理画面・通信は別） |
| **U01** | PC / モバイル / 320px / キーボード / 200% / 画像・フォント失敗 / bfcache | 可読・操作可能、横溢れなし、前 UID・遅延応答・proof が復活しない | visual / font / privacy / runtime / 実 bfcache local |
| **B01** | 実装後の Phase 1 `!app` / `!mypage` / `!page` / `!my page` | 全エイリアスで固定公開入口・UTM へ誘導（秘密・UID なし、既存 `my` オプション不変） | alias / UTM 契約と parser / workspaceapp tests |

※未実施や `false` を合格に置き換えないでください。試験ごとに範囲・リリースの組・開始終了時刻・秘匿済み資料の参照・結果（`pass` / `fail` / `pending`）を記録します。

---

## Canon S01 / S04 実経路セキュリティ検証

将来の承認済み DEV 実経路に対する最小検証手順です。まずオーナー / インフラ担当の非公開棚卸しで、対象 Hosting リリースと API 書き換え先、Cloud Run サービス・digest・稼働リビジョン、`mypage-server` の各種設定、DEV URL とログ閲覧境界を確認してください（欠落があれば `pending` として前工程へ戻します）。

1. リリースの組を固定する: [リリース記録](release-record.example.json) に承認済み DEV の Hosting リリース、書き換え先、Cloud Run リビジョン / イメージ digest、FE/BE の SHA、試験時間窓、ロールバック対象を非公開参照で固定します（試験中にリビジョンが変わった場合は結果を混ぜずにやり直します）。
2. S01（UID 単位の 429 と未認証リクエスト）
   - IP 制限は設けていません（理由は [実装仕様](../README.md) の「HTTP とキャッシュ」を参照）。
   - 承認済みの 2 名の合成ユーザー A・B を用意し、A だけが限定負荷で `GET /api/mypage` を繰り返して `429` に達すること、正の `Retry-After` が返ること、待機後に回復すること、同じ時間帯の B に波及しないことを確認します。
   - 続けて同じ合成クライアントから無効な App Check やログイン開始を一定回数繰り返し、アプリが `429` を返さないこと（通常の `4xx` で終わること）を確認します。Hosting 経由と `run.app` 直アクセスの両方で実施し、Cloud Run の最大インスタンス数・同時実行数を超える負荷は掛けません（`RemoteAddr` や `X-Forwarded-For` は記録不要）。
3. S04（callback とログの全層確認）
   - 承認済みの合成 `code` / `state` / `__session` マーカーで callback の Hosting → rewrite → Cloud Run 転送と直後の code なしリダイレクトを確認します。
   - Platform request log、CDN / Hosting アクセスログ、Cloud Run リクエストログ、アプリケーション / セキュリティログの閲覧者・保持設定を非公開環境で照合し、クエリ・Cookie・トークン・サポート参照が不要に記録・閲覧・長期保持されていないことを確認します（アプリ側のマスキングだけでは合格にせず、閲覧できない層も合格とみなしません）。
4. 結果の扱い: 漏洩や誤った 429 波及があれば 0B Gate を止め、局所修正後に同じ実経路で再検証します（Redis・WAF・分散 limiter の追加は前提にしません）。本番公開前にも同等の確認を行います。

---

## 配備・ロールバックのリハーサル

現行仕様では `pinTag` を使わず、バックエンドを先、フロントエンドを後に配備し、戻す場合はフロントエンドを先にロールバックします（スキーマとレスポンスは前後 1 世代の互換性を維持）。承認済み環境のみで以下を実施します。

1. 直前の正常な FE/BE の組、スキーマ・同意バージョン、稼働リビジョン、Hosting 書き換え先を記録します（記憶に頼らず記録から参照）。
2. 新 Backend + 旧 Frontend の状態で、6 エンドポイント・部分障害・エラー・同意バージョンを確認します（ここで壊れる場合は新 Frontend を進めません）。
3. 新 Backend + 新 Frontend でマトリックスの必須項目を確認し、適用ポリシー版と記録された同意バージョンを照合します。
4. Frontend を直前正常リリースへ戻し、Hosting 経由で新 Backend と通信できることを確認します（必要なら Backend も対応する直前リビジョンへ戻します）。
5. 元の正常な組に戻った書き換え先・リビジョン・監視回復・同意移行ウィンドウを確認します。

> [!CAUTION]
> ロールバックで保存データの削除やプライバシー保護の解除を自動的に取り消さないでください。データ・スキーマ・同意の移行に後方互換性がない場合は作業を止め、専用復旧手順をレビューします。

---

## 緊急遮断と復旧のリハーサル（未実施）

既存構成における緊急遮断の候補手段は、オーナーが対象 Cloud Run サービスの公開 Invoker（`allUsers`）を取り消す方法です。これにより OAuth・個人 API への公開経路をサーバー側で閉じつつ、別配信の Hosting 上の静的規約ページを維持します。DEV 環境で遮断と復旧の両方を実証するまで「停止可能」の Gate は合格にしません（IAM 変更は対象・時間窓・復旧対を記録した個別承認後にのみ実施）。

1. 事前保全: 対象プロジェクト・リージョン・サービス、Hosting 書き換え先、Run URL・トラフィックを読み戻し、レビュー済み FE/BE 対と IAM ポリシーの `version` / `etag` を非公開に記録します。Invoker IAM チェックが有効で、上位プロジェクトや他に迂回できる公開付与がないことを確認します。
2. 遮断の実行: イメージ更新・public apply・Hosting リリース・外部告知を止めたうえで、オーナーが `etag` 保護付きで該当サービスの `roles/run.invoker`（`allUsers`）のみを削除し、他が変わっていないことを読み戻します。反映後、Hosting 経由・直 Run ともに OAuth / 個人 API が拒否され、`/privacy`・`/terms`・`/contact` が `200` で読めることを確認します。
3. 既存処理への注意: 新規リクエスト遮断だけでは、すでに実行中の処理や発行済み Firebase セッションは止まりません。侵害や削除対応時は [全ランタイムの停止・drain](../support-delete-inventory.md) やセッション失効を別途行います（Firestore Rules をサーバー権限遮断の代用にしません）。
4. 復旧の実行: 原因修正後、復旧承認を得て同じサービスの元メンバーだけを `etag` 保護で復元します。正常経路・拒否プローブ・同意版・S01/S04 を再検証し、元ポリシーとの差分がないことを確認します。
5. **Terraform State との整合**: 一時的な IAM 差分は専用 Terraform State に対する drift になるため、遮断中に自動修復や公開パイプラインを走らせないでください。`destroy`、Secret 削除、TTL/DB 削除、認証緩和は使わないでください。
