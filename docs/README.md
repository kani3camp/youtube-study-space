# 開発・運用ドキュメント

利用者向けのコマンド案内は [docs-site](../docs-site/README.md)、各パッケージの起動・検証方法は各 README を参照してください。

| 目的 | 参照先 |
| --- | --- |
| システム構成・座席数制御・認可境界・CI | [アーキテクチャ](development/architecture.md) |
| 座席外観 V2 の互換移行 | [rollout 手順](development/seat-appearance-v2-rollout.md) |
| 広報・SNS・サムネイル制作 | [デザイン指針](design/DESIGN.md) |
| MyPage の API・集計・認証契約 | [MyPage](mypage/README.md) |
| MyPage の配備・公開条件・復旧 | [release runbook](mypage/release/README.md) |
| MyPage の個人データ削除 | [削除手順](mypage/support-delete.md)・[対象と確認事項](mypage/support-delete-inventory.md) |
| raw チャット保存の廃止 | [retirement runbook](privacy/raw-live-chat-archive-retirement-runbook.md) |
| raw チャットを含む混合バックアップの処置条件 | [GCS cleanup](privacy/gcs-raw-chat-cleanup.md) |

ここには継続して使う契約・手順・検証入力を置きます。作業途中のメモ、依頼文案、日付ごとの試験・配備報告、生成できる試算表は追加せず、対応する Issue / PR / Actions に記録します。配備状態は対象 release の証跡で確認し、仕様や合成テストから推測しません。
