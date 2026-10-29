# ServiceAccessControl — A

[Canon 04](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)に従い、`service-access-controls/{channelId}`をUser/WebAccountから独立したサーバー専用認可ストアとして使う。MyPageを最初の利用先とし、既存Botの`!block`・YouTube Ban・通常の書き込み処理は接続しない。

## 保存内容とレスポンス

チャンネルは`UC`と22文字のYouTube IDで識別する。レコードは`generation`（1以上で単調増加する整数）、`updatedAt`、`moderation`、`privacyDeletion`の4フィールドだけを持つ。各制限理由には`active`を必須とし、有効時だけ`since`、固定の`reasonCode`、64文字の小文字16進数による推測不能な参照を持つ。moderationの参照は`actionRef`、privacyDeletionは`requestRef`とする。表示名・作業名・自由文・連絡先は保存しない。

moderationのコードは`MODERATION`、`SECURITY`、`POLICY_VIOLATION`、`LEGACY_COMPATIBILITY`、privacyDeletionのコードは`PRIVACY_DELETION`とする。各制限理由は独立し、同時に有効でもよい。同じ状態・コード・参照の再適用では、レコードのリビジョンと世代を変えない。参照の変更を含む実変更では世代を増やす。

レコードがない場合と、すべての制限が無効な場合は許可する。取得・デコード・スキーマの障害は`503 TEMPORARY_UNAVAILABLE`、moderationが有効なら`403 SERVICE_ACCESS_RESTRICTED`、privacyDeletionが有効なら`403 DATA_DELETION_IN_PROGRESS`を返す。両方が有効なら削除中の表示を優先する。内部の制限理由・チャンネル・参照はAPIに出さない。旧WebAccountの`accessBlocked`はデコード互換のためだけに残す。

## MyPageでの適用箇所

検証済みcustom-provider uidを使い、WebAccountやキャッシュの参照より前に最新文書を読む。通常のチャンネル確認、ログインの消費処理、Custom Token発行、セッション完了、メタデータ更新にも適用する。サポート本人確認のチャンネル確認・confirmと公開の法的文面は利用できる。

OAuthの消費とアカウント作成・更新、firstWebLoginAt、メタデータ更新・消去では、同じFirestoreトランザクションで制御状態を読む。保存したチェックポイントを照合してから書き込み、制御状態と競合したトランザクションは再評価する。OAuth・YouTube・トークン発行は再試行可能なトランザクションの外で実行し、発行前後も最新状態を照合する。消費後の発行失敗や拒否でトランザクションを再利用しない。信頼済みの定期メタデータ削除は継続できる。

チェックポイントには世代とFirestoreの更新時刻を含める。キャッシュ・共有処理のキーにも含め、キャッシュ参照、リクエストから独立した処理、結果公開、レスポンス・代替応答で最新状態を照合する。無効状態での許可はキャッシュしない。制限ストアの障害は通常のメタデータ取得元障害と型を分け、部分取得や古い値の200レスポンスに変換しない。

フロントエンドは制限403で、現在の状態・統計・チャンネルメタデータを同期的に破棄する。世代更新と中断で旧結果を拒否し、定期取得・手動更新・表示復帰時の再試行を停止する。専用メッセージ、ログアウト、法的文面・サポート、明示的な新たなログインは残す。同じuidでbfcacheから復帰しても制限を維持する。通常503の既存の待機・古い値の表示は継続する。

## 信頼済み運用担当者用のモデレーションCLI

起動処理は`system/cmd/service-access-control`とし、ブラウザ管理APIは追加しない。`--stdin`または非公開の通常ファイルを読む`--manifest`で、16KiB以内の厳格なJSONを受け取る。既定は通信しないplanで、`--check-config`もオフラインとする。`--execute`でSDKを使い、本番では`--allow-production`も必要とする。

入力の全フィールドを必須とする。内容は`schemaVersion:1`、`target:{environment,projectID,channelID}`、`operation`（block/unblock/inspect）、`actionRef`（64文字の小文字16進数）、`reasonCode`、`confirmation:{environment,projectID,channelID,operation,actionRef}`である。confirmationは完全一致を必要とする。blockだけ固定reasonCodeを要求し、unblock/inspectでは空文字にする。未知・重複・null・大小文字違い・余分なJSONは拒否する。

実行対象は、信頼済みプロセスの`MYPAGE_ENVIRONMENT`と`GOOGLE_CLOUD_PROJECT`に結び付ける。任意のプロジェクト別名の不一致、認証情報ファイル、エミュレーターへの転送は拒否する。選択したプロジェクトの、鍵を使わないメタデータ認証だけを許可する。実行権限と実環境への接続は別途確認する。

blockは制御状態のコミット・検証後に、補助的にFirebaseセッションの失効を試みる。失効に失敗しても保護状態を戻さず、再実行で失効を再試行する。unblockはmoderationだけを消去し、privacyDeletionは保持する。unblockのactionRefは当該操作との対応情報であり、既存blockに対するcompare-and-swapではない。

報告は固定のstatus/code/state、世代、検証済みの固定理由だけを出す。対象・参照・パス・時刻・元のSDKエラーは出さない。終了コードは、0=成功・plan正常・設定正常、2=拒否、1=取得不能・出力失敗、3=制御保存後の失効の部分失敗とする。既存のエミュレーター専用`mypage-operator-dryrun`は維持する。

## 検証と残る判断事項

通常のGoテスト、race、Firestore Emulatorの原子性・制限理由の競合・旧トークン・メタデータとトークンの競合、匿名・合成認証済みクライアントからの制御文書の読み書き拒否、フロントエンドテスト、実Chromiumの合成QAで検証する。実SDK・実ユーザーデータ・配備・移行は、この受入に含めない。

Aでは制御レコードの削除・TTL・保持期限を追加せず、unblock後も無効状態のチェックポイントを維持する。これは保持期限のポリシー決定ではない。不存在→block→外部削除→不存在のABAを防ぐ世代の非再利用、全リビジョン・インスタンス・SDKの実行中処理の完了待ち、旧処理の再開後の新規利用との区切り、実行時の新たな本人確認から許容する経過時間は、Bの判断事項とする。MyPageの制限時間を待つだけで、全処理の完了を主張しない。

Bでは、サポート受付の完了と関連OAuthの消去、Bot・座席整理・定時/手動runner・BQ import、作業名派生データ・ベンダー・ログ・バックアップを含む横断一覧と関連全体の検証を必要とする。ベンダー・ログの実保持と権限は未確認である。D02の規約適合性は[公開条件](release/README.md)で確認する。D03の実値、CodeQL/security、実環境の公開前確認も残る。
