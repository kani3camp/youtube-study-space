# 人的 Privacy 運用手順

ログインできない利用者からのプライバシー請求（削除・全ログイン解除・開示）を受け付け、本人確認から実施・完了報告までを行う運用手順です。

## 全体の流れ

1. **窓口からの案内**: 公開のプライバシーポリシー・利用規約・`/contact` から、ログイン不要で返信可能な人的窓口へ案内します（連絡先の実値、D02 適合性、公開文面の承認は公開前の確認項目です）。
2. **受付の記録**: 所有者が受付時刻・依頼 ID・申告されたチャンネル・目的（`delete` / `revoke` / `disclosure`）を非公開の場所に記録し、CLI で `SupportRequest` を作成します（本文や連絡先は人的窓口に留めます）。削除期限は受付から **7 暦日** とし、チャレンジを再発行しても延長しません。
3. **本人確認（fresh YouTube OAuth）**: 発行したチャレンジに結び付いた新しい YouTube OAuth でチャンネル所有権を確認します。通常ログインや Firebase セッションだけでは代用しません（サポート目的のフローでは Custom Token や WebAccount を作成せず、通常ログインが ServiceAccessControl で制限中でも本人確認できます）。
4. **処置の実施**: 非公開 CLI と Runbook で環境・プロジェクト・チャンネル・確認済み依頼を固定し、削除・連携解除・開示を実施します（[削除手順](support-delete.md) の未完了項目を完了と偽りません）。
5. **結果の返信**: 元の返信可能な人的窓口から結果を返します（Firebase ユーザー削除後のアプリ内返信を前提にせず、`revoke` / 開示の操作と到達確認を非公開記録に残します）。

---

## 非公開の受付 CLI（`cmd/mypage-support-operator`）

`create`・`inspect`・`recover` の各操作は、`--execute` を付けたときだけ所有者の ADC / GCP IAM へ接続します（DEV は `test-youtube-study-space`、Production は `youtube-study-space`、Production 時は `--allow-production` が必須）。権限は既存のサーバー側 IAM で検証します。

- **入力形式**: パーミッション `0600` の非公開 JSON ファイルに、`requestId`（64 桁の 16 進数）、`targetChannel`、`acceptedAt`（UTC・マイクロ秒精度）を指定します。申告チャンネルは確認前の候補であり、権限を付与するものではありません（本文や連絡先はコピーしません）。
- **状態の維持**: 受付作成と `recover` は、トランザクション内で元の受付時刻と 7 日期限を維持します。

```sh
# system/ で実行（--execute なしはオフライン入力確認のみ）
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-operator --action create --environment development \
  --project test-youtube-study-space --purpose delete --request-file /private/path/intake.json

# 明示実行（handoff は新規 0600 ファイルへ保存し、標準出力には出さない）
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=test-youtube-study-space \
  go run ./cmd/mypage-support-operator --action create --environment development \
  --project test-youtube-study-space --purpose delete --request-file /private/path/intake.json \
  --execute --handoff-file /private/path/new-handoff.json
```

### 案内・照合・再発行（`recover`）

- **本人確認の案内**: `handoff` に出力されたチャレンジを、対象環境の `/login?supportChallenge=...` として依頼者へ非公開に案内し、同じ目的・チャンネルで fresh OAuth を完了してもらいます。完了後の証拠参照はサーバー側 `SupportRequest` と照合し、削除 CLI の非公開マニフェストへ転記します。
- **ACK / handoff 消失時の復旧**: 新しい受付を作り直さず、同じ `requestId` で `--action inspect` を実行して新規 `handoff` ファイルから Receipt / Revision を確認します。未検証状態であれば、入力に `expectedReceiptVersion` を加えて `--action recover` を実行し、元の期限を変えずにチャレンジを再発行します（`verified` / `claimed` / 完了済みの受付は巻き戻しません）。

※アプリ内受付/状態照会/返信 UI や独自 Operator OIDC などは撤去済みです（管理画面や複数運用者が必要になった段階で再設計します）。

---

## Google側で連携を解除した利用者への対応

公開の「お問い合わせ」ページから環境別フォームへ案内し、申告された環境・対象チャンネル・依頼目的を確認します。

- 本人確認は原則として対象チャンネルに対する新しい YouTube OAuth（fresh OAuth）で行い、通常ログインだけでは実行しません。
- Google 連携を解除済みで OAuth を利用できない場合も個別に案内し、チャンネル URL の申告だけで操作を行わないでください。
- 保存データ全体の削除を希望する場合は、[削除手順](support-delete.md) へ切り替えます。

### 手動でマイページの連携情報を取り除く（暫定運用）

この手順は、マイページ全体のメンテナンス切り替え、トークン失効までの待機（発行時刻を特定できない場合は最低 2 時間）、Firebase Console での手作業を伴う暫定運用です。正式手順化と fresh OAuth 不可時の本人確認の最低基準は、一般公開前に別途決定します。

1. **対象の確認**: 返信可能な窓口で「連携解除のみの依頼」であることと対象環境（開発: `test-youtube-study-space` / 本番: `youtube-study-space`）を確認します。Firebase Authentication と Firestore の両画面で同じプロジェクト・同じチャンネル ID を確認し、作業者を 1 名に固定して作業開始前のメンテナンス状態を記録します。
2. **メンテナンス有効化とユーザー無効化**: [メンテナンスモード](maintenance-mode.md) を有効にし、通常の個人 API と新規ログインが止まったことを確認します。切替前から進行していた読み取り・更新・トークン発行の完了を待ってから、対象 UID の Firebase Authentication ユーザーが存在すれば無効化し、読み戻して反映を確認します（確認に失敗したら先に進みません）。
3. **マイページ連携文書の削除**: Firestore の `web-accounts/{channelId}` を文書単位で削除し、消えたことを読み戻します。OAuth 一時文書のうち対象チャンネルと一致する通常ログイン用の文書が残っていれば削除します（※サポート依頼、利用制御、ライブチャット由来の `users` や作業履歴には触れません）。
4. **トークン有効期間の経過待機**: メンテナンスを維持したまま、最後に発行され得た Custom Token と ID トークンの両方の有効期間が過ぎるのを確認します。発行時刻を特定できない場合は、処理完了およびユーザー無効化の確認時点から **最低 2 時間**（Custom Token の交換期限 + ID トークンの有効期間）確保します（途中で発行や交換が判明した場合は、そのトークンの失効も待ちます。参考: [セッション管理](https://firebase.google.com/docs/auth/admin/manage-sessions) / [Custom Token](https://firebase.google.com/docs/auth/admin/create-custom-tokens)）。
5. **Auth ユーザー削除と復帰**: 対象 UID の Firebase Authentication ユーザーを削除し、Authentication と `web-accounts` の両方に対象がないことを確認します（すでに削除済みなら不存在を確認して次へ進みます）。作業者が今回有効にしたメンテナンスだけを解除し、受付元へ結果を返信します（途中で失敗した場合は完了と案内せず、対象と状態を再確認してから再開します）。

### リリース前の開発環境での動作確認

専用の試験用 YouTube チャンネルでライブチャットから作業履歴を作ったうえでマイページにログインし、別ブラウザに古いログインを残した状態で上記手順を実施して以下を確認します。

- Firestore と Firebase Authentication を読み返し、マイページのアカウント情報と紐付けが残っていないこと。
- メンテナンス解除後、古いブラウザのログインから個人 API が拒否されること（試験用チャンネルで改めて連携・ログインし直した後も、古いブラウザのセッションからは利用できないこと）。
- ライブチャット由来のチャンネル ID と既存の作業履歴は残っており、再連携したマイページで同じ作業履歴を確認できること。
- 未ログインのブラウザから環境に対応した問い合わせフォームへ到達し、運営者側で試験回答を送受信できること。
