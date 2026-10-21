# Development login verification

## Task contract

Goal: 実ログイン後の作業記録画面を確認し、失敗を修正して同じDEVで再確認する。
Invariants: Auth/App Check/Originを維持し、不明な履歴を0へ変換しない。検証済み範囲以外のcoverageを設定しない。個人情報・token・raw API/logは公開証跡へ置かない。
Non-goals: Production変更、既存データの書換え・削除、未確認の履歴backfill、dev/mainへのマージ。
Acceptance: ログイン済み表示・手動更新・アカウント表示・PC/mobileを確認。集計の取得障害と履歴の不完全性を区別して表示し、必要indexを確認する。
Verification: 該当UI回帰テスト、型・lint・build、CI、DEV配信後の同一ログインセッションによる確認。

## 2026-10-21 findings

実ログイン後の画面とアカウント表示・累計取得を確認。今日/今週/直近7日は取得不可。sourceは履歴query失敗をSOURCE_UNAVAILABLE、未検証coverageをHISTORY_INCOMPLETEとして区別するが、UIは両方を取得失敗の同じ説明へまとめていた。データがないと決めつけず、index inventoryと既存coverage契約を確認して修正する。

DEVの実index inventoryは10件、work-segmentsは1件（session/type検索用）、MyPageのuser-id/ended-at/started-at複合indexは0件であった。既存Firebase管理sourceへ必要なindexを追加し、ownerはreview済みの1件だけをDEVへadditive作成する。既存indexの削除やrules配備を行わず、READY確認後に同じログインsessionで再取得する。coverage未検証の履歴は引き続き取得不可として説明し、便宜的なepoch/0値を設定しない。
