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
