# Development Runtime の配備準備と履歴

Development 環境（`test-youtube-study-space` / `asia-southeast2`）における Runtime 初回構築時の手順と診断履歴です。

> [!NOTE]
> - 現行ソースのアプリケーション配備手順は [Backend CI](../../../docs/mypage/release/backend-ci.md) を参照してください（旧方式の固定リリース設定は履歴として Git に残るのみで、今後の更新には使いません）。
> - 最新の配備・実試験状況は [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236) と Actions の記録を確認し、過去の未配備記述から現在状態を判断しないでください。一般公開の条件は [配備手順書](../../../docs/mypage/release/README.md) に従います。

---

## コンテナイメージのビルドと検査

`system/Dockerfile.mypage-api`（API サーバー）と `system/Dockerfile.mypage-maintenance-operator`（maintenance Job）は、digest 固定の Go / distroless ベースイメージから `linux/amd64` バイナリをビルドします（CGO 無効、タイムゾーン同梱、nonroot 実行、Secret やローカル `.env` は非同梱）。

```sh
docker build --platform linux/amd64 --file system/Dockerfile.mypage-api --tag mypage-api:ci system
bash system/scripts/check-mypage-image.sh mypage-api:ci

docker build --platform linux/amd64 --file system/Dockerfile.mypage-maintenance-operator --tag mypage-maintenance-operator:ci system
bash system/scripts/check-mypage-operator-image.sh mypage-maintenance-operator:ci
```

通常 CI ではオフライン・read-only filesystem で実行し、API イメージには operator バイナリが含まれないこと、operator イメージには API サーバーが含まれないことなどを検査します（実環境 ADC・Secret 値・Cloud Run 起動の証明は含みません）。

---

## 2026-10-24 の State 整合・metrics 試験の履歴

### 既存 State の遅れ整合（serial 8 → 9）

- 実環境の generation 4（イメージ `4020ade3`）に対し、DEV State が serial 8（generation 2 / `ef81de63`）のまま遅れていたため、リモート書き込みなしで State を整合させました（[run 40065942906](https://github.com/kani3camp/youtube-study-space/actions/runs/40065942906) の停止 → [run 40072743264](https://github.com/kani3camp/youtube-study-space/actions/runs/40072743264) の env ブロック正規化順差異の特定と `match_state` 修正 → [run 40073932938](https://github.com/kani3camp/youtube-study-space/actions/runs/40073932938) で serial 9・generation 4 へ整合完了）。
- 一回限りの復旧用レシピート（`owner-state-recovery.json`）は復旧完了後に削除済みです。

### metrics 更新後の API 挙動調査と State 復旧（serial 10 → 11）

- [run 40075166738](https://github.com/kani3camp/youtube-study-space/actions/runs/40075166738) の 400 エラー調査により、Terraform provider が補う空の `value`/`value_source` を両方 JSON `oneof` へ送っていた問題を修正し、`validateOnly=true` で 200 を確認しました。
- 一時カスタムロール（`run.services.update` のみ、試験後解除）の追加承認を得て [run 40077157558](https://github.com/kani3camp/youtube-study-space/actions/runs/40077157558) で ON（generation 5）へ更新後、refresh-only JSON で資源が `prior_state.values` 側に現れることを native `terraform_data` fixture で再現・修正し、[run 40081768487](https://github.com/kani3camp/youtube-study-space/actions/runs/40081768487) で State-only 整合（serial 11）を完了しました。
- また実 API の競合試験で、v2 の古い `etag` 付き no-op PATCH が拒否されず成功することが分かったため、更新プロトコルを公式の v1 `resourceVersion` 付き PUT（1 回のみ・競合時リトライなし）へ変更しました。

---

## 実環境の構築・接続経緯

### DEV 認証情報の接続（2026-10-21）

- 実行時確認を受け、MyPage 専用の OAuth Web クライアント（固定 Hosting コールバック）と YouTube API 専用制限サーバーキーを作成し、DEV の既存 3 Secret に値を保存して version `1` / `ENABLED` を確認しました（値は GitHub / チャット / CI へ出していません）。
- Plan / Apply 両方の Environment に公開設定・同意版草案・イメージ digest・番号固定 Secret 参照を含む `MYPAGE_RUNTIME_SETTINGS_JSON` を接続しました。
- 初期 Plan での `labels=null` / `update_time` 更新や、一時 bootstrap 権限解除に伴う IAM `etag` 変化は、予定 `no-op` 時のみ許容するガードを追加して対処しました（実際の role/member/condition 変更や書き込み予定は引き続き拒否）。

### Bootstrap metadata の追従

- プロジェクト上のメタデータ読み取り専用ロール（Plan / Apply）、専用 Registry の upload/download ロール（Apply）、専用ランタイム SA への `actAs`（Apply）を最小権限で付与しました（Secret 実値・Cloud Run 作成・プロジェクト IAM 編集は付与していません）。
- 共有 IAM ポリシーの `etag` 変化のみを許容する refresh 判定と、専用 Registry（9 資源契約）の `labels=null → {}` 正規化（予定 `no-op` 時のみ）を追加しました。

### イメージと Runtime の実行順序

- Registry 適用後に `image-build` で `linux/amd64` イメージをビルドし、40 桁 source SHA の不変タグとして push して `RepoDigest` と照合します（既存タグの上書きは拒否）。
- Runtime 初回は `runtime_private`（11 資源：Registry 9 + Firestore 権限 + Cloud Run）から開始し、Hosting 接続時に `runtime_public`（12 資源：`allUsers` invoker 追加）へ進みます（`MYPAGE_RUNTIME_SETTINGS_JSON` は `runtime_settings` と同じキーを受け付け、旧 `history_coverage_json` は読み込み時に除外）。
- Cloud Run は Gen2、DEV `min=0 / max=1`、CPU 1 / 512Mi、request-based billing、startup boost なし、TCP startup probe、固定ランタイム SA を使用します。

### 専用 Registry の段階（履歴情報）

※当時は `registry-plan` / `registry-apply` を明示指定して 9 資源を導入していましたが、現在の通常運用では `Terraform`（`terraform.yml`）が `desired-stage.txt` に基づき最終状態を一括 Plan / Apply します。

- 2026-10-21 JST の Cloud Shell 再確認で、Foundation の IAM、Secret version 0 件、user-managed key 0 件、DEV Cloud Run 0 件を確認したうえで段階的に構築を進めました。
- 実際の稼働リビジョン・イメージ digest・Hosting リリース・同意版・試験結果は [release-record.example.json](../../../docs/mypage/release/release-record.example.json) に一対で記録します（OAuth コードや生ログは公開 PR / CI 成果物に残しません）。
