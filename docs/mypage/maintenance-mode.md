# MyPage メンテナンスモード v1

計画メンテナンス中にMyPageの通常利用を停止する機能です。セキュリティ事故への即時遮断装置ではありません。切替時に進行中の通常読み取りが完了する場合があります。通常書き込みは同じFirestoreトランザクション内で状態を確認し、Custom Tokenは発行直前に再確認します。Privacy / Supportの専用経路は維持します。

## 制御データと動作

各環境の`config/mypage`を制御状態の管理元とします。`maintenance-enabled`は真偽値、`updated-at`はFirestore Timestampです。制御文書がない、不正、または読み取り不能の場合は、安全のため通常利用を停止します。ブラウザから直接読み書きすることはできません。

バックエンドは保護対象の通常処理を始めるときに最新の制御状態を読み、有効なら`503 MAINTENANCE_ACTIVE`を返します。書き込みはトランザクション内でも再確認します。Firestore更新やトークン発行の直前確認により、処理中に切り替わった場合の危険な操作を防ぎます。ただし、外部のトークン発行とFirestoreの切替を完全にアトミックにはできません。

公開状態APIの応答は`{"maintenanceEnabled": true}`または`{"maintenanceEnabled": false}`だけとし、制御文書の更新日時は公開しません。

画面は保護ルートに入るときに状態を確認し、ACTIVEまたはAPIの`MAINTENANCE_ACTIVE`を受け取るとメンテナンス画面に移ります。「お知らせを見る」と「状態を再確認」を表示し、定期取得やタブ復帰時の自動確認は行いません。解除後は再読み込みか「状態を再確認」で通常画面へ戻ります。

## 初期化と切替

1. 先にDEVのreview済みMyPageイメージがArtifact Registryに存在することを確認します。GitHub EnvironmentのJSONへmaintenance用digestを書きません。Plan/Apply identityにはproject custom role `mypageMaintenanceRead`（`run.jobs.get` / `run.jobs.getIamPolicy` の2権限だけ）を恒常read-only bootstrap権限としてPlan/Apply identityへ用意します。Jobが不存在の場合だけ、コードで固定した実ビルド済みsource SHAのimmutable tagをPlan時にread-only解決してprivate digest入力にします。
2. `CI`の`operation=gcp-plan`と対象`environment`で、Core GCPとMyPageのdesired stateをまとめてPlanします。DEVの現在のdesired stageには、メンテナンス専用Service Account・WIF・単一Cloud Run Jobを含みます。差分のレビュー後だけ`operation=gcp-apply`を実行します。通常操作で`maintenance_operator` / `maintenance_upgrade`の個別モードは選びません。対象環境のdesired stageが`disabled`ならMyPage資源を変更しません。Job作成後のimageはTerraform設定JSONではなくBackend CI/CDが所有します。
3. 初回構築後は`operation=mypage-backend-release`でreview済みの同一digestをAPI Serviceとmaintenance Jobへ同期し、Service revision、Job generation、設定不変のreadbackが成功したことを確認します。その後、GitHub ActionsのMyPage Maintenance Modeを`workflow_dispatch`で実行し、対象`environment`と`enabled=false`を指定して初期化します。欠落文書も同じJobが作成します。
4. 開始時は`enabled=true`、解除時は`enabled=false`を指定します。Jobは入力を検証し、トランザクションで現在値を読んで更新します。コミット後に読み返し、結果をログに出します。同じ値への更新は成功のno-opです。
5. 実行結果とJobログを確認します。読み取り・更新・再読み取りの失敗は成功扱いにしません。失敗時は対象環境・権限・Firestore文書の状態を調査し、復旧後に同じワークフローを再実行します。

GitHubのWIF本人情報は固定Jobを実行できますが、Firestoreへの直接アクセス権は持ちません。Job自体は専用Service Accountで動きます。Firestore IAMはコレクション単位ではないため、Jobの権限はプロジェクト内の他文書にも及びます。

配備時はJobを用意し、初期状態の`enabled=false`を確認してからバックエンドとフロントエンドを配備します。旧形式の両者は新形式と互換ではないため、対応する両方を続けて配備します。切替中や古い画面を開いている利用者は状態を確認できず、再読み込みが必要になる場合があります。IaCの変更だけでは実環境への適用や初期化は行われません。
