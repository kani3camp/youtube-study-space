# 人的Privacy運用

1. 公開Privacy/Terms/contactから、ログインできない人も使える返信可能な窓口へ案内する。連絡先実値・D02適合性・公開文面の承認は公開前Gate。
2. 所有者が受付時刻・依頼ID・申告channel・目的(delete/revoke/disclosure)をprivateに記録し、SupportRequestを作成する。本文・連絡先は人的窓口に留める。削除期限は受付から7暦日、challenge再発行でも延長しない。
3. challengeに結び付いたfresh YouTube OAuthでchannel ownershipを確認する。通常loginやFirebase sessionだけで代用しない。Support purposeはCustom Token/WebAccountを作成せず、通常loginがSACで制限中でも本人確認できる。
4. private CLI/Runbookで対象environment/project/channel/verified requestを固定し、削除またはrevoke/開示を実施する。[削除手順](support-delete.md)の未解決Gateは完了と偽らない。
5. 元の返信可能な人的窓口で結果を返す。Firebase user削除後のアプリ内返信を前提にしない。revoke/開示の操作・到達確認はprivate記録に残し、未実行を完了としない。

## private受付CLI

`cmd/mypage-support-operator`の受付・inspect・recoverを維持する。入力は0600のprivate JSON、申告channelは本人確認前の対象候補であり権限ではない。SupportStoreの作成/recoveryはトランザクションで冪等性と期限を維持する。現時点のCLIはoffline preview/mockのみ。実接続は削除運用と併せてowner-controlled ADC/IAM・非公開handoff・環境分離で行う。未接続の経路を実稼働と扱わない。

アプリ内受付/状態照会/返信、Google operator challenge/PKCE/JWKS/ID token authority、reply/complete用独自権限frameworkは撤去した。管理画面や複数operatorが必要になった段階で再設計する。既存GitHub/GCP identityを、ユーザーのfresh OAuth本人確認と混同しない。
