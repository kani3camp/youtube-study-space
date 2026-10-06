# MyPage

React / TypeScript / Vite の独立した frontend package。実装中の基盤 slice であり、公開・認証・個人データ取得はまだ接続していない。

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

CI は `mypage/**` の変更を上記の検証に分類する。運用 credentials は検証に不要。
