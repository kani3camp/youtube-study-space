# MyPage Phase 0A–1 整理（Lean 化）の記録

> [!NOTE]
> この文書は統合ブランチ（`integration/mypage-canon-20261006`）上で行った実装・検証範囲の整理（Lean 化）の記録、および検証責任の分担をまとめたものです。実環境への配備や一般公開の承認を示すものではありません。

## 整理の方針

- 目的: 現在の基本表示と安全な人的プライバシー運用に必要な実装・検証だけを保守対象に残します。
- 維持する不変条件: fresh YouTube OAuth によるチャンネル UID 識別、Firebase Auth / App Check、IDOR 拒否、ServiceAccessControl（SAC）の独立理由と fail-closed、削除中の再生成防止、古いレスポンスの破棄、7 暦日削除と再開可能性、環境分離、秘密情報の非公開。
- 対象外（Non-goals）: 本番配備・実データ削除・一般公開承認・`dev` へのマージ、および Premium 機能・詳細分析・管理画面 UI。

---

## 集約した変更（#1482 〜 #1494 → #1489）

開始コミット `50719cc18deb7c30b66cdb6bf0d63ab3c900aa02` 時点の `dev` 差分（430 ファイル・+61,357 / -1,547 行）から、約 114 ファイルを整理し約 11,550 行の純減となりました（集約後：約 395 ファイル・+49,800 / -1,546 行）。

| PR | 整理・削減した保守対象 |
| --- | --- |
| [#1482](https://github.com/kani3camp/youtube-study-space/pull/1482) | MyPage 専用 FSL 3 モデルと関連テストを撤去（Seat 2 モデルと conformance は維持） |
| [#1483](https://github.com/kani3camp/youtube-study-space/pull/1483) | DEV 明細 API / UI、暗号化カーソル、tier、quota / lease / policy を撤去（月次・原記録 writer / rebuild は維持） |
| [#1484](https://github.com/kani3camp/youtube-study-space/pull/1484) | アプリ内プライバシー受付・返信 UI と未接続の Operator OIDC / PKCE / JWKS を撤去（人的窓口と fresh OAuth は維持） |
| [#1485](https://github.com/kani3camp/youtube-study-space/pull/1485) | 独自の App Check JWT / JWKS / cache / singleflight を撤去し、公式 Admin SDK + 薄い境界ラッパーへ集約 |
| [#1486](https://github.com/kani3camp/youtube-study-space/pull/1486) | Terraform provider の computed 属性全体の allowlist と個別 drift 判定を撤去（重要設定・操作・State・Plan 整合の検証は維持） |
| [#1487](https://github.com/kani3camp/youtube-study-space/pull/1487) | ビルド内の `tsc` と重複していた独立した Typecheck ステップを削除 |
| [#1488](https://github.com/kani3camp/youtube-study-space/pull/1488) | 作業契約と検証責任の整合 |
| [#1490](https://github.com/kani3camp/youtube-study-space/pull/1490) | 公式 SDK 試験フィクスチャの lint 修正 |
| [#1491](https://github.com/kani3camp/youtube-study-space/pull/1491) | Registry / Catalog / PreparedDeletion・合成アダプター・mock ワークフローを撤去し、現行ストアへ直接接続 |
| [#1492](https://github.com/kani3camp/youtube-study-space/pull/1492) | 人的受付 CLI を固定環境の所有者 ADC / IAM と新規 `0600` handoff ファイルへ接続 |
| [#1493](https://github.com/kani3camp/youtube-study-space/pull/1493) | 記録文書と旧コメントの整合 |
| [#1494](https://github.com/kani3camp/youtube-study-space/pull/1494) | 非公開 checkpoint のランダム owner 保持と排他ロックにより二重起動を拒否 |

---

## 維持する境界と検証の責任分担

MyPage 専用 FSL は実装 conformance 未接続のため撤去し、以下の実装テストで各リスクを担保しています（既存の `SeatSession` conformance と `SeatAllocation` モデルは維持）。

| リスク（Failure mode） | 主な検証層 |
| --- | --- |
| OAuth 目的混同・二重 consume・発行中の失効 | Go `auth` / `support auth` / `access` Emulator integration |
| ログアウト・UID 変更・`pagehide` 後の旧レスポンス復活 | Vitest `memory` / `runtime` |
| `moderation` / `privacyDeletion` の独立性・古い許可の再使用 | `serviceaccess` unit / Firestore integration |
| ブラウザからサーバー専用データへのアクセス | Firestore Rules allow / deny |
| 削除対象の取り違え・部分更新・古い世代での再開 | Firestore transaction integration・checkpoint unit |
| Auth UID の再作成・停止/再開確認の欠落 | `current-store` integration |
| BigQuery の ACK 消失・SQL 対象混入・削除後の残存 | 実 SDK を使うローカル HTTP 試験・パラメータ / `COUNT` 再読 |

- App Check の扱い: 公式 Admin Go SDK へ委譲し、SDK が検証しない正確な `issuer` / project number / Web App ID / `iat` や claim 型 panic の安全な拒否だけを薄いラッパーで補っています（起動時 JWKS 取得と鍵更新の実可用性は実環境での確認事項です）。

---

## 削除運用であえて残した設計

詳細は [削除手順書](support-delete.md) と [人的 Privacy 運用](support-operator.md) を参照してください。

- 固定 29 checkpoint と短い実行ループ: ストアごとの削除 → 再確認、停止 → drain → 再開の位置を保持し、ACK 消失時に先頭から新規データを再削除しないようにしています（任意のワークフロー登録機能は持ちません）。
- checkpoint ファイルの owner 保持と排他ロック: 同一ファイルの二重起動を拒否し、別ファイルによる横入りもストア側の owner 照合で停止させます。消失・破損時は全旧実行の停止を確認する所有者回復まで保護を解除しません。
- owner / revision / generation / CAS と証跡結合: SAC 変更や別実行からの割り込みを検出し、保護解除を最終トランザクションで確定します。
- Auth creation timestamp の一時記録: 削除後に同じ UID で再作成されたアカウントを古い削除実行へ巻き込まないようにします（最終トランザクションで消去）。
- 手動確認項目: SDK で個別削除や不存在を証明できない実コピーおよび全ランタイムは、合成アダプターで完了済みとみなさず、所有者による手動確認項目として残します。
- 本番ロールバックの方針: 削除済みデータの復元ではなく、既知サービスリビジョンへの復帰、writer 停止、`privacyDeletion` 維持、同一 checkpoint からの前進復旧を組み合わせます。

---

## Terraform と CI の整理

- Terraform: provider が返す computed 属性全体の独自 allowlist を撤去し、型の妥当性は Terraform 本体の validate / mock test / plan に委ねつつ、重要な宣言値・許可リソース操作・State owner・保存済み Plan の HMAC 照合に集中しています。
- CI: 通常の MyPage 変更では元から FSL ジョブを動かしていないためルーティングは広げず、`mypage` ビルド内の `tsc` と重複していた Typecheck ステップを削除しました。

---

## ローカル検証の記録（整理完了時点）

※GitHub Actions の実行結果は `#1489` の Checks を参照してください（ローカル実行と Actions 実行は分けて扱います）。

| 検証項目 | 結果と補足 |
| --- | --- |
| MyPage Vitest | 207 件 → 整理後 195 件成功（撤去機能の専用テスト 12 件を削除。認証・メモリ・ランタイム・基本表示は維持） |
| MyPage check / build | 成功（ビルド内で TypeScript 型検査も実施） |
| FSL | MyPage 3 モデル撤去、Seat 2 モデル + Go conformance 成功 |
| Terraform Python contract | 87 件 → 整理後 74 件成功（撤去した computed metadata テストを削除） |
| Terraform native | DEV / PROD 各 21 mock tests、`fmt` / `validate`、foundation / metrics plan 成功 |
| Go | 全 unit 成功、`golangci-lint`（CI 指定設定）0 issues |
| Firestore Emulator / Rules | 全スイート成功（追加の current-store integration で対象隔離・UID 再作成拒否・手動 resume を確認） |
| i18n | `I18N_BASELINE=ja go generate ./...` 成功（意図しない差分なし） |
| CI routing / contract | パス検出成功、既存 CI 契約テスト 22 件成功 |

---

## 公開前に残っている実環境での確認事項と Phase 2

- 実環境での確認事項: DEV の fresh OAuth / App Check / Hosting 実経路、鍵更新可用性、Canon S01 / S04、原記録と集計結果の実データ照合、モニター / OBS 認証、費用と監視通知、D02 / D03・ログイン不要の人的窓口と返信到達、backup / vendor / log 棚卸し、最小 IAM、7 暦日削除と本番ロールバックの実受入（今回、実環境での CLI 実行・配備・実データ削除は行っていません）。
- Phase 2 での再設計: 詳細履歴・Premium 分析、アプリ内プライバシー受付/返信、複数運用者向け OIDC / UI は仕様確定時に一から再設計します（今回撤去した独自フレームワークや FSL をそのまま戻す前提にはしません）。
