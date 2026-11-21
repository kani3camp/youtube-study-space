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
  - 一時的な除外: `package.json` の `pnpm.auditConfig.ignoreGhsas` で [GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf)（picomatch 4.0.7 以下の ReDoS、修正版なし）だけを除外しています。picomatch は `vite` / `@tailwindcss/vite` 経由のビルド時依存で、リポジトリ内の固定パターンにだけ使われます。外部から glob パターンを受け取る経路はなく、配信物にも含まれません。修正版が出た時点か勧告が取り下げられた時点で、除外を外して依存を更新してください。除外を増やすときも、同じように理由と外す条件をここに書きます。
- コンテナ検査: MyPage のビルドイメージは Trivy で OS とライブラリの High / Critical を検査します（修正版のない指摘も自動除外せず、検査失敗や脆弱性 DB 取得失敗を成功とみなしません）。
- Dependabot の扱い: [`.github/dependabot.yml`](../.github/dependabot.yml) では MyPage フロントエンド（`/mypage` の npm）を日次更新、バックエンドの Go modules（`/system` 等）および Docker ベースイメージ（`/system` 等）を月次更新として設定しています。ただし、MyPage 向けの更新設定はリポジトリの既定ブランチへマージされてから有効になるため、統合ブランチ上の CI 成功だけで自動更新が始まっているとはみなさないでください（PR #1236 の完了時に更新先ブランチも見直します）。
