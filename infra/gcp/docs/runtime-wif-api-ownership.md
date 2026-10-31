# GCP Terraform runtime WIF / API ownershipの移行記録

既存資源のTerraform取り込みはDEV・PRODとも完了し、移行専用の実行経路は撤去しました。
現行の通常Plan/Applyは[README](../README.md#通常のdev-terraform開発)と[認証・実行境界](terraform-authentication-boundary.md)、
State復旧は[State account recovery](state-account-recovery.md)を参照してください。

## 定常Plan/Applyへの移行（2026-10-16）

当時の固定manifestによるimport/no-op・空State初期化・計算観測同期の経路は、一般のState復旧機能ではありません。
通常資源の追加後は当時の固定baselineに一致しなくなるため、復旧用の常設コードとして保持しません。
移行用job・controller・契約テストとlive E2Eは削除し、通常運用に必要な入力・State・provider検証だけを残しています。

既存managed Secretのschema名・圧縮transport・4field形式は`terraform_runtime.py`で互換性を維持します。
移行用GCP identity/WIF、GitHub Secret、実Stateの削除や変更は、このコード撤去に含みません。
旧sourceを再利用する場合も、現行State・HCL・IAMとの適合を新しいPRで確認します。

## 移行記録

過去の判断・失敗・実行結果は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)、
[PR #1251](https://github.com/kani3camp/youtube-study-space/pull/1251)、
[撤去前の固定Source](https://github.com/kani3camp/youtube-study-space/blob/0a2a825de73618735384cd34afe3f407a229824b/infra/gcp/docs/runtime-wif-api-ownership.md)で追跡できます。
履歴のツール・入力・成功記録は、現在の実行承認や現在の実環境の証明には使いません。
