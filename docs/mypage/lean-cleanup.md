# MyPage Phase 0A–1 Lean化

## 作業契約

- Goal: 現在の基本表示と安全な人的Privacy運用に必要な実装・検証だけを保守する。
- Invariants: fresh YouTube OAuth/channel UID、Firebase Auth/App Check、IDOR拒否、SACの独立理由とfail closed、削除中の再生成防止、旧response破棄、7暦日削除・再開可能性、環境分離、秘密非公開を維持する。
- Non-goals: Production配備・実データ削除・一般公開承認・devへのmerge、Premium/詳細分析/admin UI。
- Acceptance: FSL/DEV明細/無効intake/未接続operator OIDCの撤去、公式SDK中心のApp Check、運用対象に限定した削除処理、Terraform/CIの重複解消、関連検証成功。
- Verification: Go unit/lint/generate、MyPage check/test/build、Firestore Emulator/Rules、Terraform fmt/validate/testと危険操作guard、CI routing。実環境QAはローカル試験とは別Gate。

## ブランチと統合順

目的別cleanup PR → `refactor/mypage-lean-20261029` → [#1489](https://github.com/kani3camp/youtube-study-space/pull/1489) → `integration/mypage-canon-20261006` → #1236 (`dev`、エージェントmerge禁止)。各sliceはcleanupブランチにのみ集約し、#1489と#1236のmergeは実施しない。

## 集約した変更

| PR | 保守対象の削減 |
| --- | --- |
| [#1482](https://github.com/kani3camp/youtube-study-space/pull/1482) | MyPage FSL 3モデル・model regression/mutation。Seat 2モデルとconformanceは維持 |
| [#1483](https://github.com/kani3camp/youtube-study-space/pull/1483) | DEV明細API/UI・encrypted cursor・tier・quota/lease/policy。月次/原記録writer/rebuildは維持 |
| [#1484](https://github.com/kani3camp/youtube-study-space/pull/1484) | app内Privacy受付/状態/返信・未接続Google Operator OIDC/PKCE/JWKS/authority。人的窓口/fresh OAuthは維持 |
| [#1485](https://github.com/kani3camp/youtube-study-space/pull/1485) | 独自App Check JWT/JWKS/cache/singleflight。公式Admin SDK + 固定project/app/時刻/fail closed wrapperへ集約 |
| [#1486](https://github.com/kani3camp/youtube-study-space/pull/1486) | provider computed schema全体のallowlistとmetadata drift個別判定。実操作/重要設定/State/Plan commitmentを維持 |
| [#1487](https://github.com/kani3camp/youtube-study-space/pull/1487) | build内tscと重複した独立Typecheck step |
| [#1488](https://github.com/kani3camp/youtube-study-space/pull/1488) | 作業契約・検証責任の整合 |
| [#1490](https://github.com/kani3camp/youtube-study-space/pull/1490) | 公式SDK試験fixtureのlint修正 |
| [#1491](https://github.com/kani3camp/youtube-study-space/pull/1491) | Registry/Catalog/PreparedDeletion、合成Auth/BQ/vendorアダプターとsnapshot連動、実行用mock workflowを撤去し現在storeへ直接接続 |
| [#1492](https://github.com/kani3camp/youtube-study-space/pull/1492) | 人的受付CLIを固定環境の所有者ADC/IAM・新規0600 handoffへ接続 |

開始commitは`50719cc18deb7c30b66cdb6bf0d63ab3c900aa02`。同じ`dev`基準の差分は開始時430ファイル・追加61,357行/削除1,547行から、集約後約395ファイル・追加49,700行/削除1,546行へ変化した。cleanup自体は約112ファイル・追加1,550行/削除13,200行、純減約11,650行（最終報告の文言変更を除く概算）。主眼は未使用のUI・認証方式・合成store・独自parser・provider schema二重管理を保守対象から外すことにある。

## 維持する境界と検証の責任

| Failure mode | 主な検証層 |
| --- | --- |
| OAuth目的混同・二重consume・mint中の失効 | Go auth/support auth/access Emulator integration |
| logout/uid変更/pagehide後の旧response復活 | Vitest memory/runtime |
| moderation/privacyDeletionの独立性・古い許可の再使用 | serviceaccess unit/Firestore integration |
| ブラウザからserver専用データへのアクセス | Firestore Rules allow/deny |
| 削除対象の取り違え・部分更新・古い世代での再開 | Firestore transaction integration・checkpoint unit |
| Auth UIDの再作成・停止/再開確認の欠落 | current-store integration |
| BQのACK消失・SQL対象混入・削除後の残存 | 実SDKを使うローカルHTTP試験・parameter/COUNT再読 |

MyPage専用FSLは実装conformance未接続のため撤去した。上記の実装試験を維持し、SeatSession conformanceとSeatAllocationの独立モデルは維持する。

App Checkは公式SDKの実APIを確認して置換した。SDKが固定しない正確なissuer/project number/Web App/iat等を薄いwrapperで確認し、SDKのclaim型assertion panicも認証拒否へ変換する。SDK管理のJWKS更新を採用し、独自の鍵cache/再取得状態機械を持たない。新しい鍵の反映タイミングと起動時JWKS取得はSDKの挙動に従うため、実環境の鍵更新可用性は別Gateとする。

## 削除運用の形

詳細は[削除Runbook](support-delete.md)と[人的窓口](support-operator.md)。verified SupportRequestとfresh OAuthを初回transactionで確保し、privacyDeletion・proof claim・checkpointを同時に記録する。対象を固定した現在Firestore、channel UIDのAuth、固定BQへ直接処理を行う。各storeを再読し、全writer再開を所有者が確認した後の最終transactionだけが関連参照をscrubして保護を解除する。moderationは独立して残る。

BFF cache破棄、全writer停止/drain、古いqueue/import/restore排除、legacy mapping、backup/export、vendor/派生物、logは所有者Runbookの期限付きprivate Gateで照合する。宣言ファイルを自動処理や第三者検証の証拠とは扱わない。unknownや7暦日要件に反する残存コピーは完了を妨げる。

あえて残した複雑性:

- 固定29 checkpointと短い実行ループ: store別delete/inspect、停止/drain/resumeの再開位置を保持し、ACK消失で先頭から新規データを再削除しない。任意workflowの登録機構は持たない。
- owner/revision/generation/CAS・証跡結合: SAC変更や別実行からの横入りを検出し、保護解除をtransactionで確定する。
- Auth creation timestampの一時記録: UID再作成を古い削除実行へ取り込まない。最終transactionで消去する。
- 手動Gate: SDKで個別削除・不存在を証明できない実コピーや全runtimeを、合成adapterで完了済みと偽らない。
- 月次summary writer/coverage/rebuildとmigration rollback: 基本表示と既存データ移行に必要。詳細履歴UIとは分離する。

Production rollbackは削除済みユーザーデータの復元を意味しない。既知サービスrevisionへの復帰、writer停止、privacyDeletion維持、同じcheckpointからの前進復旧を組み合わせる。

## TerraformとCI

providerが返すcomputed属性全体の独自schema/unknown allowlistを撤去した。Terraform native validate/mock test/planへ任せる部分と、重要な宣言値・許可したresource操作・State owner・saved-plan/HMACを確認する部分を分けた。public IAM、Secret値の混入、他State、delete/replace、想定外update、metrics/security driftは引き続き拒否する。provider metadataの全項目をテストで再列挙しない。

通常MyPage変更では元々FSL jobを選択しておらず、routingを広げない。MyPage buildがtscを実行するため独立Typecheck stepを削除した。Go unit/Emulator/Rulesは異なるfailure modeを検証するため維持する。認証付きGCP操作は手動operationとして区別し、通常PR成功から実環境Readyを判定しない。meta-CIは追加していない。

## ローカル検証記録

| 検証 | 変化・結果 |
| --- | --- |
| MyPage Vitest | 開始ソース207成功 → cleanup後195成功。撤去surface専用12件を削除し、auth/memory/runtime/基本表示を維持 |
| MyPage check/build | 成功。build内でTypeScriptも検証 |
| FSL | MyPage 3モデル撤去、残るSeat 2モデル + Go conformance成功 |
| Terraform Python contract | 開始時87件 → 74件成功。撤去したcomputed metadata contract専用試験を削除 |
| Terraform native | DEV/PROD各21 mock tests、fmt/validate、foundation/metrics plan成功 |
| Go | 最終コードの全unit成功、CI指定golangci-lintで0 issues |
| Firestore Emulator/Client Rules | 全suite成功。追加current-store integrationで対象隔離・UID再作成拒否・完了後再実行・手動resumeを確認。resume時刻修正後も対象integration成功 |
| i18n | `I18N_BASELINE=ja go generate ./...` 成功、意図しない生成diffなし |
| CI routing/contract | path detector成功、既存CI contract 22件成功 |

GitHub Actionsの結果は#1489のChecksを別途参照する。ローカル実行とActions実行を混同しない。

## 公開前に残す実環境GateとPhase 2

DEVのfresh OAuth/App Check/Hosting実経路、鍵更新の可用性、S01/S04、履歴coverage実データ照合、monitor/OBS認証、費用/監視通知、D02/D03・ログイン不要の人的窓口と返信到達、backup/vendor/log inventory、最小IAM、7暦日削除とProduction rollbackの実受入は対象リリースで確認する。初回削除確保前のfresh OAuth証跡がTTL消失済みなら、証拠を推測せず元受付期限内で本人確認を解決する。旧合成checkpointの実在有無も配備前に確認する。

CLIの実装とローカル試験だけで一般公開Readyとは判断しない。今回、実環境でのCLI実行・配備・実データ削除は行っていない。

Phase 2で仕様が確定した場合のみ、詳細履歴/Premium analytics、app内Privacy受付/返信、複数operator用OIDC/UIを再設計する。今回撤去した独自frameworkやFSLを、そのまま戻す前提にはしない。
