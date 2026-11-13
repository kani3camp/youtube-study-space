# 人的Privacy運用

1. 公開Privacy/Terms/contactから、ログインできない人も使える返信可能な窓口へ案内する。連絡先実値・D02適合性・公開文面の承認は公開前Gate。
2. 所有者が受付時刻・依頼ID・申告channel・目的(delete/revoke/disclosure)をprivateに記録し、SupportRequestを作成する。本文・連絡先は人的窓口に留める。削除期限は受付から7暦日、challenge再発行でも延長しない。
3. challengeに結び付いたfresh YouTube OAuthでchannel ownershipを確認する。通常loginやFirebase sessionだけで代用しない。Support purposeはCustom Token/WebAccountを作成せず、通常loginがSACで制限中でも本人確認できる。
4. private CLI/Runbookで対象environment/project/channel/verified requestを固定し、削除またはrevoke/開示を実施する。[削除手順](support-delete.md)の未解決Gateは完了と偽らない。
5. 元の返信可能な人的窓口で結果を返す。Firebase user削除後のアプリ内返信を前提にしない。revoke/開示の操作・到達確認はprivate記録に残し、未実行を完了としない。

## private受付CLI

`cmd/mypage-support-operator`のcreate・inspect・recoverは、`--execute`時だけ所有者のADC/GCP IAMへ接続する。DEV/Productionのprojectは削除CLIと同じ固定mapping。Productionでは`--allow-production`を必要とする。Google Operator OIDCやCLI指定operator名では権限を作らず、server-side Firestore権限は既存IAMが検証する。

入力は0600のprivate JSON。`requestId`（64桁hexの安定ID）、`targetChannel`、`acceptedAt`（UTC/microsecond精度）を指定する。申告channelは本人確認前の対象候補であり権限ではない。受付作成とrecoverはtransactionで同じ元受付時刻・7日期限を維持する。本文・連絡先をコピーしない。

```sh
# system/から。初回は--executeなしでoffline input確認。
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-operator --action create --environment development \
  --project test-youtube-study-space --purpose delete --request-file /private/path/intake.json

# 明示実行。handoffは新規0600ファイルへだけ保存し、stdoutには出さない。
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-operator --action create --environment development \
  --project test-youtube-study-space --purpose delete --request-file /private/path/intake.json \
  --execute --handoff-file /private/path/new-handoff.json
```

handoffのChallengeを対象環境の`/login?supportChallenge=...`へprivateに案内し、同じ目的・channelのfresh OAuthを完了する。証跡参照はサーバー側SupportRequestと照合し、削除CLIのprivate manifestへ転記する。通常loginを依頼本人確認の代用にしない。

受付ACK/handoff消失時は新しい受付を作らず、同じrequestIdで`--action inspect`し、新規handoffファイルからReceipt/Revisionを確認する。未検証なら`expectedReceiptVersion`を入力へ加えて`--action recover`し、元の期限を変えずchallengeを再発行する。verified/claimed/完了済み受付を再発行で巻き戻さない。revoke/開示の実処置は人的private Runbookで証跡を残し、受付作成だけで完了としない。

アプリ内受付/状態照会/返信、Google operator challenge/PKCE/JWKS/ID token authority、reply/complete用独自権限frameworkは撤去した。管理画面や複数operatorが必要になった段階で再設計する。既存GitHub/GCP identityを、ユーザーのfresh OAuth本人確認と混同しない。

## Google側で連携を解除した利用者への対応

公開の「お問い合わせ・請求の案内」から環境別フォームへ案内する。受付後、申告された環境・対象チャンネル・依頼目的を確認する。本人確認は原則として、依頼の対象と同じチャンネルを新しいYouTubeの許可で確認する（fresh OAuth）。通常のログインだけで依頼を実行しない。Google連携を解除済みなどでこの確認を利用できない場合は個別に案内し、チャンネルURLの申告だけで操作しない。保存データ全体の削除を希望する場合は、既存の削除手順に切り替える。

### 手動でマイページの連携情報を取り除く

この手順は、マイページ全体のメンテナンス、トークン失効までの待機（発行時刻を特定できない場合は最低2時間）、Firebase Consoleでの手作業を伴う暫定運用である。正式手順化と、fresh OAuthを利用できない場合の本人確認の最低基準は、公開前に別途判断する。

1. 返信可能な窓口で、解除だけの依頼であることと対象を確認する。開発は `test-youtube-study-space`、本番は `youtube-study-space`。Firebase AuthenticationとFirestoreの両画面で同じプロジェクトと同じチャンネルIDを確認する。作業者を一人に固定し、作業開始前のメンテナンス状態を記録する。
2. [メンテナンスモード](maintenance-mode.md)を有効にし、通常の個人APIと新規ログインが停止したことを確認する。切替前からのMyPageの読み取り・更新・トークン発行と配信が完了するまで待つ。対象UIDのFirebase Authenticationユーザーが存在する場合は無効化し、無効化を読み返す。無効化または実行中処理の確認が失敗した場合は先に進まない。
3. Firestoreの `web-accounts/{channelId}` を文書単位で削除して読み返す。OAuthの一時文書のうち、対象チャンネルと一致する通常ログイン用の文書が残っていれば削除する。サポート依頼、利用制御、ライブチャット由来のユーザー・履歴には触れない。
4. メンテナンスを維持し、最後に発行され得たCustom Tokenと、最後に取得され得たIDトークンの両方の有効期間が過ぎたことを確認する。発行時刻を特定できない場合は、処理の完了と、既存ユーザーがいる場合はその無効化を確認した時点から、Custom Tokenの交換可能期間と交換後のIDトークンの有効期間を合わせて少なくとも2時間確保する。途中で発行や交換が判明した場合は、そのトークンの有効期間も過ぎるまで解除しない。[Firebaseのセッション管理](https://firebase.google.com/docs/auth/admin/manage-sessions) · [Custom Token](https://firebase.google.com/docs/auth/admin/create-custom-tokens)
5. 対象UIDのFirebase Authenticationユーザーを削除し、Authenticationと `web-accounts` の両方に対象がないことを再確認する。削除済みなら不存在を確認して次へ進める。作業者が今回有効にしたメンテナンスのみ解除する。受付元へ結果を返信し、途中で失敗した場合は完了と案内せず、同じ依頼の対象・状態を確認してから再開する。

### リリース前の開発環境での確認

専用の試験用YouTubeチャンネルで、ライブチャットから作業履歴を作ったうえでマイページにログインする。別ブラウザーの古いログインを残し、上記の操作を実施する。

- FirestoreとFirebase Authenticationを読み返し、マイページのアカウント情報とひも付けが存在しない。
- メンテナンス解除後に古いログインから個人APIが拒否される。試験用チャンネルで改めて連携してログインした後も、古いブラウザーのログインから個人APIを利用できない。
- ライブチャット由来のチャンネルIDと既存の作業履歴が残り、改めて連携したマイページで同じ作業履歴を確認できる。
- 未ログインのブラウザーから環境に対応したフォームへ到達し、運営者が管理する試験回答の送信と受信を確認する。
