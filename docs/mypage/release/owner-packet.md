# MyPage owner preparation packet

これは本人へ一度で説明するための準備案。新しい実フォーム・公開・Google設定変更は実行していない。[Privacy Canon](https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf)と[09 D01〜03](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f)に従う。D01方式は2026-10-07、D02設計選択は2026-10-09確定済み。D02の規約適合性確認、D03の未決値と実運用Gateを残す。

## 指定済みのOAuthサポートメール

OAuth同意画面の user support email は、**指定済みのOAuthユーザーサポート用メール**を使用する設定案とする。指定はOAuth同意画面での表示用途に限る。実値は私的contextと安全なoperator設定入力で保持し、この公開repoには記載しない。Privacy窓口、一般問い合わせ、developer contactへの兼用や他目的の公開は決定していない。Google Auth Platformへの実設定変更・権限変更は別承認とし、このpacketでは実行しない。

運営者の本名・自宅住所をPrivacy本文へ常時掲載しない既定を維持し、必要な請求へ遅滞なく回答する運用を用意する。公開サービス名/名称を表示する場合も、実名を勝手に推測・要求しない。公開draftの氏名placeholderは、常時実名掲載が必須という決定ではない。

## Privacy請求の受付準備

一般問い合わせは既存の公開案内を継続する。Privacy請求の通常入口は[アプリ内受付](../privacy-intake.md)に変更した。既存の有効なFirebase本人識別がある場合にだけ自動受付し、対象channelはserverのuidから決める。受付と7暦日期限の記録をfresh OAuthより先に行い、本人確認成功でも削除・開示・全session解除は実行しない。通常MyPageの利用制限中も、この狭い請求経路は使える。

利用可能な既存sessionがない場合は、公開Contactに設定する**返信可能な人的例外窓口**へ案内する。匿名の自動SupportRequest作成、推測した公開メール、OAuth scope追加は行わない。例外窓口の媒体・担当・期限管理・本人確認手順・実URLは未確定で、D03の公開文面とともに確認する。アプリ内status/返信は本人確認済みrecordと同一Firebase uidの両方を必要とし、session喪失や削除完了後は人的窓口で対応する。公開前にoperator返信の認可・監査・保持/削除と実通知運用を成立させる。

案内文draft:

> 個人情報の取り扱い、保存データの削除・開示等、ログインの解除は、利用可能なログインがある場合、アプリ内から依頼できます。受付後、対象チャンネルを新しいYouTubeの許可で本人確認します。受付や本人確認だけでデータは削除されません。ログインできない場合も、公開された返信可能なPrivacy窓口へご連絡ください。認証token・パスワード・身分証等を送らないでください。

## D01・D02の確定仕様と残る確認

| 項目 | 既決 | 残る証跡・確認 | 公開前の条件 |
| --- | --- | --- | --- |
| D01 削除中停止 | 2026-10-07確定。User/WebAccountとは独立したServiceAccessControlでmoderationとprivacyDeletionを独立管理 | 初期適用はMyPageのみ。cache前のauthoritative read、write transaction直前の再確認、取得障害は503 fail closed。Bot未接続期間の実削除はdestructive windowにrelevant legacy writerを全体pause/drainする | 方式の再判断は不要。全runtime/launch・store・restore inventoryと実pause/drain/横断削除の証跡、個別の実行承認は未完 |
| D02 Google grant取消 | 2026-10-09確定。外部Google解除検知だけを目的とするrefresh tokenの要求・保存と定期pollingは採用しない。Firebase sessionとgrant取消は別 | 実際の短命OAuth→channel identity→Firebase session→public metadata/cleanup flowをverification/complianceへ提示し、取消後に保持済みAPI dataをどう扱うべきか規約解釈を確認する | 規約適合性は未確認。一般OAuth verificationだけをこの解釈の承認と読み替えず、設計選択の確定を公開承認と扱わない |

D03のうちOAuth user support emailの指定は上記で解消済み。返信窓口・実公開URL・施行日・管轄など、残る値は公開版へ整合させる。現時点で追加質問を繰り返さず、公開準備時にまとめて実値と本文をreviewする。アプリ内受付の合成検証や公開URLの形式検査はD01の実装・D02の適合性確認・D03の実値確定の証拠ではない。

## OAuth / Hostingの安全な先行準備

今、本人が先に用意できるものは、選ぶ公開domain/予定URL、本人テスト用Google accountと対象channel、Privacy返信窓口の媒体と担当/期限管理、施行日/管轄の公開条件。D01・D02の設計選択は再質問せず、D01の実装・実運用証跡とD02の規約適合性を確認する。既存一般フォームは継続、実名/自宅住所の常時掲載とOAuthサポートメールは再質問しない。インフラの設定値やIAMは担当の証跡で閉じ、新しい本人質問へ置換しない。

- App Name/サービス説明、Homepage/Privacy/Terms予定path、user support email（指定済み、実値は安全なoperator設定入力で受け渡す）、未指定のdeveloper contact、所有domainとGoogle Auth PlatformのTesting accounts候補を整理する。指定メールの用途を拡張しない。
- scopeは既決の `youtube.readonly` のみ。offline access/refresh tokenを追加しない。既存scope justificationと[demo storyboard](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)を実装と照合する。
- callbackは固定 `/api/auth/youtube/callback`。Google account/Brand Account/複数channelの本人テスト対象を用意する。誤channelを自動confirmしない。
- public Firebase Web App・App Check/reCAPTCHAの登録値、keyless signer、Hosting target/Cloud Run rewrite、CSP通信inventoryはowner/infra担当の証跡から受け取る。デフォルト値や既存project名から推測しない。
- GA4は現行explicit opt-in、User-ID/Signals/adsなし、保持14か月・reset OFF。Console設定と実Networkで確認するまでreadiness宣言で有効化しない。
- Phase 1のBot入口は `!app` / `!mypage` / `!page` / `!my page` と固定UTMを正本どおり準備する。新しいruntimeコマンド接続は本PRに含めず、別scope承認後に実装し、Phase 1承認まで実URLを有効化しない。

実行時に別承認を求めるもの: project/API/secret/IAM/App Check/Auth/Hosting変更、resource作成・deploy、実ユーザー/production dataへの操作、Google Auth PlatformのIn production切替と審査提出、Privacy受付の有効化・公開、0B受入とPhase1告知。設定案・ドラフト作成・synthetic local検証は先行できる。

## 本人へ一度で依頼する文案（review用）

> 公開前の準備として、D01・D02の確定事項、D02の適合性確認とD03の残る実値をまとめて確認してください。OAuth同意画面のサポートメールは指定済み、本名/自宅住所は常時非公開、一般問い合わせは既存フォーム継続として準備しています。指定メールの実値は安全なoperator設定入力で扱い、公開資料には掲載しません。
>
> 1. D01は独立ServiceAccessControl方式に確定済みです。初期MyPageのguardに加え、Bot未接続期間の実削除では関係legacy writerの全体pause/drainが必要です。方式の選択は再依頼せず、実inventory・再生成防止・横断削除・復元後の再削除を証跡で確認します。moderationとinactive generation checkpointは削除完了後も保持する実装です。
> 2. D02は、外部Google解除検知だけのためのrefresh token保存と定期pollingを採用しない設計に確定しました。実際の認証/session/metadata flowと取消後の保持済みAPI dataの扱いをverification/complianceへ提示し、規約適合性を確認します。この確認が終わるまで公開Gateは閉じません。
> 3. 返信可能でログインを必要としないPrivacy窓口の媒体・担当・期限管理、希望する公開domain/予定URL、本人テストaccountと対象channelを用意してください。アプリ内受付はsource-onlyで無効のままです。例外窓口とoperator返信の実運用が整うまで有効化しません。施行日・管轄等は公開予定と本文のreview時に確定します。
>
> インフラ担当から受け取るproject/app/IAM等の証跡は別に進めます。Privacy受付の有効化・公開、OAuth実設定・権限変更、live試験、deployや実データ操作は、対象と具体的な操作を確認してから承認を受けて実施します。

この文案は準備依頼のためのもの。D01・D02の設計確定を実運用承認やGate完了へ読み替えず、D02の適合性確認・D03の実値確定・公開承認・外部送信は未実施とする。
