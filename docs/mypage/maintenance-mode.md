# MyPage メンテナンスモード v1

計画メンテナンス中に MyPage の通常利用を停止するための機能です。セキュリティ事故への即時遮断装置ではありません。切替時に進行中の通常 read が完了する場合があります。通常 write は同じ Firestore transaction 内で状態を確認し、Custom Token は発行直前に再確認します。Privacy / Support の専用経路は維持します。

## 制御データと動作

各環境の `config/mypage` が正本です。`maintenance-enabled` は boolean、`updated-at` は Firestore Timestamp です。制御文書がない、壊れている、または読み取れない場合、通常利用は安全側に停止します。ブラウザからこの文書を直接読み書きすることはできません。

Backend は通常の保護対象処理を開始するときに最新の制御を読み、enabled の場合は `503 MAINTENANCE_ACTIVE` を返します。書き込みは transaction 内で再確認します。Firestore や token 発行の状態変更の直前確認は、処理中に切り替わった場合の危険な操作を防ぐためです。外部 token 発行と Firestore 切替の完全な原子性はありません。

公開 status API の応答は `{"maintenanceEnabled": true}` または `{"maintenanceEnabled": false}` のみです。制御文書の更新日時は公開しません。

画面は保護ルートに入るときに状態を確認し、ACTIVE または API の `MAINTENANCE_ACTIVE` でメンテナンス画面へ遷移します。「お知らせを見る」と「状態を再確認」を表示します。自動 polling やタブ復帰時の自動確認は行いません。解除後は利用者が再読み込みするか「状態を再確認」を押して通常画面へ戻ります。

## 初期化と切替

1. 対象環境と承認済み image digest を確認し、Terraform の `maintenance_operator`（既存公開 runtime がある場合は `maintenance_upgrade`）で専用 Service Account、WIF と単一 Cloud Run Job を準備します。実際の plan / apply は別途承認が必要です。
2. GitHub Actions **MyPage Maintenance Mode** の `workflow_dispatch` で対象 `environment` と `enabled=false` を指定して初期化します。欠損 document の作成も同じ Job が行います。
3. 開始時は `enabled=true`、解除時は `enabled=false` を指定します。Job は入力を検証し、transaction で現在値を読んで更新し、commit 後に読み返して結果をログに出力します。同じ値は成功する no-op です。
4. 実行の成功・失敗と Job ログを確認します。読取・更新・readback の失敗は成功扱いになりません。失敗時は対象環境と権限、Firestore の文書状態を調査し、復旧後に同じ workflow を再実行します。

GitHub からの WIF identity は固定 Job を実行でき、Firestore の直接アクセス権は持ちません。Job 自体は専用 Service Account で動作します。Firestore IAM は collection 単位ではないため、Job identity の権限はプロジェクト内の他の document にも及びます。

配備時は Job を用意し初期 `enabled=false` を確認してから Backend と Frontend を配備します。旧形式を使う Backend と Frontend は新形式と互換ではないため、対応する両方を続けて配備します。切替中や古い画面を開いたままの利用者は状態を確認できず、再読み込みが必要になる場合があります。IaC の変更だけでは実環境への適用や初期化は行われません。
