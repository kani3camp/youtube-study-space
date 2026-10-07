# MyPage owner preparation packet

これは本人へ一度で説明するための準備案。新しい実フォーム・公開・Google設定変更は実行していない。[Privacy Canon](https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf)と[09 D01〜03](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f)の未決を残す。

## 指定済みのOAuthサポートメール

OAuth同意画面の user support email は、本人指定の **kani3camp@gmail.com** を使用する設定案とする。この用途での公開表示は指定済み。Privacy窓口、一般問い合わせ、developer contactへの兼用や他目的の公開は決定していない。Google Auth Platformへの実設定変更・権限変更は別承認とし、このpacketでは実行しない。

運営者の本名・自宅住所をPrivacy本文へ常時掲載しない既定を維持し、必要な請求へ遅滞なく回答する運用を用意する。公開サービス名/名称を表示する場合も、実名を勝手に推測・要求しない。公開draftの氏名placeholderは、常時実名掲載が必須という決定ではない。

## 窓口の準備案

一般問い合わせは[既存Google Formを案内する公開ページ](https://app.notion.com/p/673257758f8849dcb5cdeafeb160ee8c)を継続する。ここでフォーム内部設定を調査・変更済みとは扱わない。Privacy専用Google Formの採用・実URL・作成/公開は未確定。媒体の可否判断と、返信・本人確認・期限管理ができる運用の成立を分ける。

Privacy Formを選ぶ場合のdraft:

| 項目 | 推奨する準備案 | 理由 |
| --- | --- | --- |
| アクセス | Google/本サービスのログインを必須にしない | ログイン不能時や運営情報への請求も受け付ける |
| 返信先 | 回答できるemailを本人入力。必要な案内だけに利用する説明 | 受付・fresh proof案内・完了連絡が必要 |
| 依頼分類 | Privacy問い合わせ / 保存データ削除 / 全session解除 / 開示・訂正等 | logout、Google grant取消、削除を区別する |
| 内容 | 依頼内容、必要な場合だけ対象channelのURL/申告 | 申告だけを本人確認にしない。不要な身分証/作業履歴は集めない |
| 設定 | 返信の通知・担当の監視・受付時刻/期限記録、応答の公開一覧OFF | 7日SLAの起算と個人data保護 |
| 完了案内 | 受付済みであり、本人確認/実行/完了とは別と説明 | form送信やproof成功で削除完了を表示しない |
| 運用 | form response/通知/担当copyの保存先・access・保持/削除をinventoryへ追加 | 受付dataも削除/保持対象に含める |

必須にしないもの: FAQ読了、通常ログイン、既存session、本人のスクショ、過剰な身分資料。自動収集のGoogle emailやfile uploadは必要性とdata flowの確認前に追加しない。ログインできない例外は公開窓口で個別対応する。

案内文draft:

> 個人情報の取り扱い、保存データの削除・開示等、ログインの解除についてはこちらへご連絡ください。受付後、必要に応じて対象チャンネルを新しく確認する方法をご案内します。フォーム送信や本人確認だけでデータを削除することはありません。ログインできない場合も、この窓口へご連絡ください。認証token・パスワード・身分証等を送らないでください。

既存の本人確認は、受付後のSupportRequestとfresh OAuthをenvironment/request/purpose/channelへbindingする専用flow。opaqueな参照番号は照合用で、削除権限でも公開ticket URLでもない。削除受付を「ログイン後の専用フォームで完結」に変更しない。

## D01 / D02の必要判断

| 項目 | 既決 | 推奨案と代案 | 必要な承認 |
| --- | --- | --- | --- |
| D01 削除中停止 | 7日以内横断削除、通常revokeは最大約1時間、proofは実行と別 | channel単位の一時guardを全Bot/batch/BFF/refresh/cacheへ適用し、世代/checkpointで古い遅延処理を拒否。代案は関係サービスを全体停止し残存token/遅延処理を待つ | 0B前に方式を本人確定。全writer実装/実store inventory/実行対象をreviewした後に個別の実行承認 |
| D02 Google grant取消 | YouTube refresh tokenは要求/保存なし。Firebase sessionとgrant取消は別。既定R-API受容あり | 実際の短命OAuth→channel identity→Firebase session→public metadata/cleanup flowをverification/complianceへ提示し確認。回答前0Bを選ぶ代案は新しい残余リスク受容と再設計条件の明示 | 一般OAuth verificationだけをこの解釈の承認と読み替えない。新しいリスク受容を代理確定しない |

D03のうちOAuth user support emailの指定は上記で解消済み。返信窓口・実公開URL・施行日・管轄など、残る値は公開版へ整合させる。現時点で追加質問を繰り返さず、公開準備時にまとめて実値と本文をreviewする。フォーム案や設定の形式検査はD01/D02/D03全体完了の証拠ではない。

## OAuth / Hostingの安全な先行準備

今、本人が先に用意できるものは、選ぶ公開domain/予定URL、本人テスト用Google accountと対象channel、Privacy返信窓口の媒体と担当/期限管理、施行日/管轄の公開条件、D01/D02の方針判断。既存一般フォームは継続、実名/自宅住所の常時掲載とOAuthサポートメールは再質問しない。インフラの設定値やIAMは担当の証跡で閉じ、新しい本人質問へ置換しない。

- App Name/サービス説明、Homepage/Privacy/Terms予定path、user support email（kani3camp@gmail.com）、未指定のdeveloper contact、所有domainとGoogle Auth PlatformのTesting accounts候補を整理する。指定メールの用途を拡張しない。
- scopeは既決の `youtube.readonly` のみ。offline access/refresh tokenを追加しない。既存scope justificationと[demo storyboard](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)を実装と照合する。
- callbackは固定 `/api/auth/youtube/callback`。Google account/Brand Account/複数channelの本人テスト対象を用意する。誤channelを自動confirmしない。
- public Firebase Web App・App Check/reCAPTCHAの登録値、keyless signer、Hosting target/Cloud Run rewrite、CSP通信inventoryはowner/infra担当の証跡から受け取る。デフォルト値や既存project名から推測しない。
- GA4は現行explicit opt-in、User-ID/Signals/adsなし、保持14か月・reset OFF。Console設定と実Networkで確認するまでreadiness宣言で有効化しない。
- Phase 1のBot入口は `!app` / `!mypage` / `!page` / `!my page` と固定UTMを正本どおり準備する。新しいruntimeコマンド接続は本PRに含めず、別scope承認後に実装し、Phase 1承認まで実URLを有効化しない。

実行時に別承認を求めるもの: project/API/secret/IAM/App Check/Auth/Hosting変更、resource作成・deploy、実ユーザー/production dataへの操作、Google Auth PlatformのIn production切替と審査提出、Privacy Form作成/公開、0B受入とPhase1告知。設定案・ドラフト作成・synthetic local検証は先行できる。

## 本人へ一度で依頼する文案（review用）

> 公開前の準備として、次の3点をまとめて確認してください。OAuth同意画面のサポートメールは指定済みの kani3camp@gmail.com、本名/自宅住所は常時非公開、一般問い合わせは既存フォーム継続として準備しています。
>
> 1. 削除中の停止方式は、対象チャンネルだけを一時停止し、Bot/batch/API/cacheと遅延処理に共通のguardを適用する案を推奨します。関係サービス全体を停止する代案との運営上の選択をお願いします。選択後に実装・実inventory・削除検証を行います。
> 2. Google側の許可取消の扱いは、実際の認証/session/metadata flowをverification/complianceへ提示して確認する案を推奨します。回答前に0Bを選ぶ場合は、追加残余リスクと再設計条件の明示が別途必要です。
> 3. 返信可能でログインを必要としないPrivacy窓口の媒体・担当・期限管理、希望する公開domain/予定URL、本人テストaccountと対象channelを用意してください。Privacy Formは上記の項目案までで、作成/公開はまだ行いません。施行日・管轄等は公開予定と本文のreview時に確定します。
>
> インフラ担当から受け取るproject/app/IAM等の証跡は別に進めます。フォーム作成・公開、OAuth実設定・権限変更、live試験、deployや実データ操作は、対象と具体的な操作を確認してから承認を受けて実施します。

この文案は準備依頼のためのもので、選択済み・承認済み・外部へ送信済みとは扱わない。
