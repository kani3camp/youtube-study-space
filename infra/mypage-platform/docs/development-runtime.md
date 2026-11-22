# Development Runtime の配備準備と履歴

Development 環境（`test-youtube-study-space` / `asia-southeast2`）における Runtime 初回構築時の手順と診断履歴です。

> [!NOTE]
> - 現行ソースのアプリケーション配備手順は [Backend CI](../../../docs/mypage/release/backend-ci.md) を参照してください（旧方式の固定リリース設定は履歴として Git に残るのみで、今後の更新には使いません）。
> - 最新の配備・実試験状況は [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236) と Actions の記録を確認し、過去の未配備記述から現在状態を判断しないでください。一般公開の条件は [配備手順書](../../../docs/mypage/release/README.md) に従います。

---

## コンテナイメージのビルドと検査

`system/Dockerfile.mypage-api`（API サーバー）と `system/Dockerfile.mypage-maintenance-operator`（maintenance Job）は、digest 固定の Go / distroless ベースイメージから `linux/amd64` バイナリを 1 つずつビルドします（CGO 無効、タイムゾーン同梱、nonroot 実行、Secret やローカル環境ファイルは非同梱）。

```sh
docker build --platform linux/amd64 --file system/Dockerfile.mypage-api --tag mypage-api:ci system
bash system/scripts/check-mypage-image.sh mypage-api:ci

docker build --platform linux/amd64 --file system/Dockerfile.mypage-maintenance-operator --tag mypage-maintenance-operator:ci system
bash system/scripts/check-mypage-operator-image.sh mypage-maintenance-operator:ci
```

通常 CI ではネットワーク無効・read-only filesystem で実行し、API イメージでは `--check-config` で設定なし拒否・合成設定受理・nonroot・operator バイナリ非同梱を、operator イメージでは nonroot・引数なし拒否・API サーバー非同梱を確認します（実環境 ADC・Secret 値・Google OAuth・Cloud Run 起動の証明は含まず、CI で image push や cloud 認証は行いません）。

---

## 2026-10-24 の State 整合・metrics 試験の履歴

### 既存 State の遅れ整合（serial 8 → 9）

実環境の generation 4（イメージ `4020ade3`、開始時は generation 3）に対し、DEV State が serial 8（generation 2 / `ef81de63`）のまま遅れていたため、リモート書き込みなしで State のみを整合させました（固定 DEV 12 資源・全 no-op・UID / etag / Ready / traffic 不変を維持）。

- [run 40065942906](https://github.com/kani3camp/youtube-study-space/actions/runs/40065942906) は旧 State との差を `runtime-refresh-fields` で拒否して停止し（State 事前照合を修正）、一回限りの復旧用レシピート（`owner-state-recovery.json`）とその検証経路は完了後に削除済みです。
- 復旧ソースの [run 40072743264](https://github.com/kani3camp/youtube-study-space/actions/runs/40072743264) で env ブロックの provider 正規化順と REST 挿入順のみの差による `metrics-live-config-mismatch` 停止を確認し、`match_state` で重複・空名を拒否して env を名前順に比較するよう修正しました。
- [run 40073932938](https://github.com/kani3camp/youtube-study-space/actions/runs/40073932938) で独立 Plan/HMAC・保存 Plan 適用・live 照合に成功し、独立 AWS 読み戻しでも 12 資源・serial 9・generation 4（`4020ade3`）・サービス UID 不変を確認しました。

### metrics 更新後の API 挙動調査と State 復旧（serial 10 → 11）

- [run 40075166738](https://github.com/kani3camp/youtube-study-space/actions/runs/40075166738) の 400 エラー調査で、provider が補う空の `value`/`value_source` を両方 JSON `oneof` へ送っていた問題を修正し、Cloud Run 公式 `validateOnly=true` で 200 を確認しました。
- 既存 Apply SA が `run.services.update` を持たないことを確認したうえで、US$1 以内の DEV 試験に限定して同権限 1 個だけの一時カスタムロールを固定 `mypage-api` に付与・試験後解除する追加承認を得ました（IAM member の etag のみの State refresh 完了後に試験を実施）。
- [run 40077157558](https://github.com/kani3camp/youtube-study-space/actions/runs/40077157558) で ON（generation 5 / `mypage-api-00005-hgk`）へ更新した際、refresh-only 検証が `metrics-readback-resources` で停止しました（独立 AWS 読み戻しは serial 10 / generation 4）。native `terraform_data` fixture により、refresh-only JSON では `planned_values.root_module` ではなく `prior_state.values` 側に資源が現れることを再現・修正し（途中失敗 `40079459702` は再現なしのため原因断定せず）、[run 40081768487](https://github.com/kani3camp/youtube-study-space/actions/runs/40081768487) で 1 回の metrics 切替のみを許す State-only 整合（serial 11 / generation 5）を完了しました。
- 実 API の競合試験では v2 の古い `etag` 付き no-op PATCH が拒否されず成功して出力メタデータのみ進んだため、更新方式を公式の v1 `resourceVersion` 付き PUT（v2 Plan と照合後、`status` を除いた snapshot へ metrics env 1 個のみ変更して 1 回限り送信・競合時リトライなし）へ変更しました。no-op 試験で変わった `etag` / `update_time` / `last_modifier` の 3 出力は別の通常 no-op Plan でのみ反映します。

---

## 実環境の構築・接続経緯

### DEV 認証情報の接続（2026-10-21）

- 実行時確認を受け、MyPage 専用の OAuth Web クライアント（固定 Hosting コールバック）と YouTube API に限定したサーバーキーを作成し、DEV の既存 3 Secret へ値を保存してすべて version `1` / `ENABLED` をメタデータのみで確認しました（値は GitHub / チャット / CI へ非公開）。
- Plan / Apply 両 Environment に公開設定・同意版草案・イメージ digest・3 個の番号固定 Secret 参照のみを含む `MYPAGE_RUNTIME_SETTINGS_JSON` を接続しました（実 OAuth / E2E / 文面公開承認の完了は意味しません）。
- Runtime public の [run 39546080459](https://github.com/kani3camp/youtube-study-space/actions/runs/39546080459) と [run 39547114678](https://github.com/kani3camp/youtube-study-space/actions/runs/39547114678) は `check=contract` で停止したため（空きローカル State では成功）、実行フェーズを区別して固定ラベルで診断できるよう整備しました。
- 初回 [Runtime private Plan 39543529630](https://github.com/kani3camp/youtube-study-space/actions/runs/39543529630) が `registry-refresh-empty-labels-only` で停止したため（イメージ push により Registry 更新時刻が進んでいた）、同一固定 Registry の予定 `no-op` で空 labels 表現差と `update_time` の単調 RFC3339 UTC 増加（ナノ秒逆行や予定 write は拒否）のみ許容するガードを追加しました。一時 bootstrap 権限解除に伴う共有 IAM `etag` 変化も予定 `no-op` 時のみ許容します。

### Bootstrap metadata の追従

- プロジェクト上のメタデータ読み取り専用カスタムロール（Plan / Apply）、専用 Registry の upload/download ロール（Apply）、専用ランタイム SA への `actAs`（Apply）を最小権限で付与しました（Secret 実値・Cloud Run 作成・プロジェクト IAM 編集は非付与）。
- 旧ガードの [run 39527611450](https://github.com/kani3camp/youtube-study-space/actions/runs/39527611450) と [run 39528628841](https://github.com/kani3camp/youtube-study-space/actions/runs/39528628841) は `metadata-refresh-address` で停止したため（独立 State 読み戻しで serial=3 と Registry の `labels=null` を確認）、当該 4 member の共有 IAM `etag` 変化と専用 Registry（9 資源契約）の `labels=null → {}` 正規化を、いずれも予定 `no-op` 時のみ許容する判定を追加しました。

### イメージと Runtime の実行順序

- Registry 適用後に `image-build` で `linux/amd64` イメージをビルドし、40 桁 source SHA の不変タグとして push して `RepoDigest` と照合します（既存タグの上書きは拒否）。
- 当時の Runtime 初回は旧 `runtime-private` 段階（11 資源：Registry 9 + Firestore 権限 + Cloud Run）から開始し、Hosting 接続時に旧 `runtime-public` 段階（12 資源：`allUsers` invoker 追加）へ進みました。現在は段階の選択が無く、`MYPAGE_RUNTIME_SETTINGS_JSON` の有無とオーナーの公開 Invoker の実在から宣言範囲が決まります（[README](../README.md#宣言範囲を決めるのは-terraform-外の事実だけ)。同 JSON は `runtime_settings` と同じキーを受け付け、旧 `history_coverage_json`・`privacy_version`・`terms_version` は読み込み時に除外）。
- Cloud Run は Gen2、DEV `min=0 / max=1`、CPU 1 / 512Mi、request-based billing、startup boost なし、TCP startup probe、固定ランタイム SA を使用します（Invoker IAM check やアプリの Auth / App Check / Origin / SAC は維持し、前後 1 世代互換や rollback は [実経路 E2E](../../../docs/mypage/release/e2e.md) で別途確認します）。

### 専用 Registry の段階（履歴情報）

※当時は `registry-plan` / `registry-apply` を明示指定して 9 資源（既存 Foundation 8 資源 + 専用 Registry 1 資源、初回 create 権限は 1 資源作成・読み戻し後に解除）を導入していましたが、現在は段階の選択も `desired-stage.txt` も無く、`Terraform`（`terraform.yml`）が環境の宣言を一括 Plan / Apply します。

- 2026-10-21 JST の Cloud Shell 再確認で、Foundation の IAM、Secret version 0 件、3 SA の user-managed key 0 件、DEV Cloud Run 0 件（同 region の既存 Artifact Registry は `gcf-artifacts` のみ）を確認しました（Google Auth Platform の自動作成 Web クライアントは流用しません）。
- 実際の稼働リビジョン・イメージ digest・Hosting リリース・同意版・試験結果は [release-record.example.json](../../../docs/mypage/release/release-record.example.json) に一対で記録します（OAuth コードや生ログは公開 PR / CI 成果物に残しません）。
