# MyPage Phase 0A–1 Lean化

## 作業契約

- Goal: 現在の基本表示と安全な人的Privacy運用に必要な実装・検証だけを保守する。
- Invariants: fresh YouTube OAuth/channel UID、Firebase Auth/App Check、IDOR拒否、SACの独立理由とfail closed、削除中の再生成防止、旧response破棄、7暦日削除・再開可能性、環境分離、秘密非公開を維持する。
- Non-goals: Production配備・実データ削除・一般公開承認・devへのmerge、Premium/詳細分析/admin UI。
- Acceptance: FSL/DEV明細/無効intake/未接続operator OIDCの撤去、公式SDK中心のApp Check、運用対象に限定した削除処理、Terraform/CIの重複解消、関連検証成功。
- Verification: Go unit/lint/generate、MyPage check/test/build、Firestore Emulator/Rules、Terraform fmt/validate/testと危険操作guard、CI routing。実環境QAはローカル試験とは別Gate。

## ブランチと統合順

`refactor/mypage-lean-*` → `refactor/mypage-lean-20261029` → `integration/mypage-canon-20261006` → #1236 (`dev`、エージェントmerge禁止)。cleanup PRは目的別に作成し専用ブランチへ集約する。

## 検証の責任

| Failure mode | 主な検証層 |
| --- | --- |
| OAuth目的混同・二重consume・mint中の失効 | Go auth/support auth/access Emulator integration |
| logout/uid変更/pagehide後の旧response復活 | Vitest memory/runtime |
| moderation/privacyDeletionの独立性・古い許可の再使用 | serviceaccess unit/Firestore integration |
| ブラウザからserver専用データへのアクセス | Firestore Rules allow/deny |

MyPage専用FSLは実装conformance未接続のため撤去した。上記の実装試験を維持し、SeatSession conformanceとSeatAllocationの独立モデルは維持する。

## 集約済みの変更

| PR | 保守対象の削減 |
| --- | --- |
| #1482 | MyPage FSL 3モデル・model regression/mutation。Seat 2モデルとconformanceは維持 |
| #1483 | DEV明細API/UI・encrypted cursor・tier・quota/lease/policy。月次/原記録writer/rebuildは維持 |
| #1484 | app内Privacy受付/状態/返信・未接続Google Operator OIDC/PKCE/JWKS/authority。人的窓口/fresh OAuthは維持 |
| #1485 | 独自App Check JWT/JWKS/cache/singleflight。公式Admin SDK + 固定project/app/時刻/fail closed wrapperへ集約 |
| #1486 | provider computed schema全体のallowlistとmetadata drift個別判定。実操作/重要設定/State/Plan commitmentを維持 |
| #1487 | build内tscと重複した独立Typecheck step |

通常MyPage変更では元々FSL jobを選択しておらず、routingを広げない。FSL reportは残るSeatモデルだけを列挙する。Go unit/Emulator/Rulesは各々別のfailure modeを担い、認証付きGCP operationは通常PR validationの成功から実環境Readyと判定しない。

## 検証記録

- 開始commit `50719cc18deb7c30b66cdb6bf0d63ab3c900aa02` のMyPage Vitest: 207成功。cleanup後: 195成功、check/build成功。減った12件は明細/受付専用で、auth/memory/runtime/基本表示の試験を維持する。
- Seat FSL 2モデル + Go conformance成功。
- MyPage Terraform: 74 Python contract tests、release tests、DEV/PROD各21 native Terraform mock tests、fmt/validate、native foundation/metrics plan成功。危険なresource操作・metrics/security drift・public IAM・State分離・Secret参照のみの境界を維持する。
- 既存Firestore Emulator/Client Rules suite、Go全unit tests、CI routing、既存CI contract 22件成功。削除系の追加変更は別途その最終ソースで再検証する。
- `I18N_BASELINE=ja go generate ./...` に意図しない生成diffなし。

## 公開前に残す実環境Gate

ローカル/合成試験は一般公開Readyの証明ではない。DEVのfresh OAuth/App Check/Hosting実経路、鍵更新の可用性、S01/S04、履歴coverage実データ照合、monitor/OBS認証、費用/監視通知、D02/D03・人的窓口、backup/vendor/log inventory、7暦日削除とrollbackの実受入は対象リリースで確認する。今回の作業は配備・実データ削除を含めない。

Phase 2で仕様が確定した場合のみ、詳細履歴/Premium analytics、app内Privacy受付/返信、複数operator用OIDC/UIを再設計する。
