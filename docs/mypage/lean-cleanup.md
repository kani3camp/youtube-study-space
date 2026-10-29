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
