# DEV recorded history display

## Task contract

Goal: 保存済みの正の作業記録を、履歴完全性の未検証だけで7日グラフから消さない。
Invariants: 完全な集計のworkSecとavailabilityは維持する。未検証の0、取得失敗、上限超過、矛盾を確定値にしない。Auth・identity・snapshot/asOf・read-only・privacy・既存fontを維持する。
Non-goals: 未検証coverage設定、履歴backfill、既存データの変更、新規DB/ジョブ、Production、dev/main merge。
Acceptance: HISTORY_INCOMPLETEでも正常に取得・検証できた正の作業時間だけを任意のrecordedWorkSecで渡し、グラフとサマリーは「記録済み分・集計未確定」と区別する。0/失敗/上限/矛盾には付与しない。旧client/serverとの前後1世代互換を維持する。
Verification: backendの日境界・休憩除外・異常系・overflow、OpenAPI/fixture、client validation・private memory・表示の回帰、通常CI、DEV実経路の反映と320/390/1440px確認。

## Findings (2026-10-22 JST)

ログイン中のaccountに対応するDEVの閉じた作業記録は存在し、時刻とdurationの整合を確認できた。Cloud Runの履歴coverage設定は未指定で、集計はHISTORY_INCOMPLETEを返していた。古い詳細履歴とlifetimeには差があり、存在する記録や最古の記録だけから完全性を宣言しない。実account、ID、個別作業時間、raw responseはprivate診断証拠だけに保持する。

完全性を保証したworkSecは引き続きnull/unavailable。追加のrecordedWorkSecは取得済みの有効な区間の正の合計であり、未知の区間を0に置いた完全な合計ではない。値がない日は従来どおり不明とし、日別の0を新たに生成しない。取得・上限・整合性の障害ではrecordedWorkSec自体を返さない。
