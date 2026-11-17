# MyPage フロントエンド

React / TypeScript / Vite で構築された独立フロントエンドパッケージです。認証・データ取得・同意管理を担うクライアント・ランタイムおよび Vitest テストを含みます。

- 実装仕様: [MyPage 実装仕様](../docs/mypage/README.md)
- 表示の確認: [画面表示のリファレンスと確認手順](../docs/mypage/design/README.md)
- 設定検査・実環境 E2E・公開条件: [配備・公開準備手順](../docs/mypage/release/README.md)
- 最新の統合・検証状況: [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)

## ローカル検証コマンド

運用認証情報なしで以下を実行できます。

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

CI では `mypage/**` の変更に対して上記の検証を実行します。

## 依存関係とビルドのセキュリティ検査

- Node.js / 脆弱性検査: Node はルートの `.node-version` / `.nvmrc` を使用します。CI では開発依存も含めて `pnpm audit --audit-level=high` を実行し、High / Critical を検出した場合は失敗させます（Go 側は `govulncheck -test ./...`）。
- コンテナ検査: MyPage のビルドイメージは Trivy で OS とライブラリの High / Critical を検査します（修正版のない指摘も自動除外せず、検査失敗や脆弱性 DB 取得失敗を成功とみなしません）。
- Dependabot の扱い: MyPage フロントエンド・Go・Docker の日次更新設定はリポジトリの既定ブランチへマージされてから有効になるため、統合ブランチ上の CI 成功だけで日次運用が始まっているとはみなさないでください（PR #1236 の完了時に更新先ブランチも見直します）。
