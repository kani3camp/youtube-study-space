# MyPage archive

過去の設計比較と単発の開発・検証メモ。記載されたHEAD、配備前の状態、保留事項は当時の記録であり、現在の運用手順として使用しない。

現行の入口は[実装契約](../README.md)、[視覚・操作の受入条件](../design/README.md)、[release手順](../release/README.md)。実装・DEV配備・試験の最新状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。

## 設計比較

- [Approved原本と追跡済みexportの関係・比較記録](visual-reference.md)
- [旧メインprototype](approved-prototype.dc.html)
- [旧状態別gallery](verification.dc.html)

HTMLは両方とも同梱されていない`support.js`を参照し、承認原本のhashとも異なる。参照履歴として内容を保持し、製品の依存関係や実行可能なテストとしては扱わない。

## 単発の確認記録

- [ログイン・履歴indexと理由別表示](development-login-check.md)
- [チャンネル画像のCSP inventory](development-avatar-check.md)
- [小さい文字とfont描画の比較](development-text-check.md)
- [不完全履歴の記録済み分表示](development-history-check.md)
