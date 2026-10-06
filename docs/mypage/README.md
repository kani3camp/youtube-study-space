# MyPage implementation contract

現行仕様から実装に必要な非機密 wire contract だけを抽出したもの。`openapi.yaml` は six-endpoint API の契約、`fixtures/` は実ユーザーに依存しない合成例。

- 本人識別は YouTube channel ID と同じ Firebase custom-provider uid。client から対象 ID を受け取らない。
- OAuth callback の atomic claim と confirm の atomic consume の後に、外部 API / token mint を transaction callback の外で実行する。
- 現在・今日・月曜開始の週・生涯累計・直近7暦日は同じ snapshot / `asOf` に揃える。
- seconds を合計し、表示時だけ分に切り捨てる。取得障害・不完全履歴・矛盾は正常な0に置き換えない。
- account metadata と作業統計の部分障害を分離し、raw ID / credentials / DB path を response・log に載せない。

契約検証:

```sh
python -m pip install -r docs/mypage/contract-requirements.txt
python .github/scripts/check-mypage-contract.py
```

Frontend と backend は独立して検証する。provisioning / production deployment はこの実装 stack に含まない。

## Web authentication storage

`oauth-transactions/{opaque id}` と `web-accounts/{verified channel id}` を server-only に追加する。既存 rules の default deny により browser access は許可しない。10分の期限は application が判定し、TTL 設定・権限・cleanup の実環境設定は provisioning gate 後に別途行う。

Callback は `pending -> processing -> channel_verified`、confirm は `channel_verified -> consumed`。WebAccount upsert と consume は同じ transaction。firstWebLoginAt は authenticated session completion で set-if-absent。users / 作業履歴は web login によって変更しない。transaction record には state hash・同意版・公開 channel metadata・時刻だけを保存し、code / OAuth token / Custom Token は保存しない。

Google provider と Firebase token mint は interface として分離し、demo emulator では合成 provider を使用する。本番への結線・デプロイは未実施。

## Pure aggregation

`WorkSnapshot` は同じ Firestore snapshot から読む adapter との境界。snapshot read time がない入力は受理しない。`Coverage` は完全性を確認した期間だけを指定し、未設定なら登録済み利用者の履歴を `HISTORY_INCOMPLETE` とする。実環境の coverage 開始日・欠損範囲は未確認であり、推測設定しない。

現行 writer は退室時に `users.TotalStudySec` へ入室セッション分を転記する。そのため入室中の lifetime は `TotalStudySec + SeatDoc.CumulativeWorkSec + work 状態の経過秒`。履歴の ongoing 区間は `CurrentSegmentStartedAt` を使い、作業名変更による closed segment と重ねない。break 中もすでに完了した session work は lifetime に残す。

1001件・取得失敗・重複/未来/負の区間・coverage 欠損を正常な0へ変換しない。独立して正しい current / lifetime / account は保持する。この slice は pure calculation の検証であり、実 DB の read-only snapshot adapter と既存 writer の競合整合性検証は次の slice。

## Contract completeness

現時点の6 endpointは通常login用の最初の契約slice。問い合わせ本人確認の `supportChallenge` / purpose binding と Custom Tokenを発行しないsupport confirmは後続契約であり、login-only schemaを最終対応版とは扱わない。
