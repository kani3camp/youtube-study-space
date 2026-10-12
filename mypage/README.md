# MyPage

React / TypeScript / Vite の独立した frontend package。認証・取得・同意のclient/runtimeと合成QAを実装済み。実Firebase/OAuth/Hostingへの結線・公開は別Gateで、公開文面はdraftを維持する。[release準備](../docs/mypage/release/README.md)で設定preflight・本人準備・実E2Eの完了条件を確認する。

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

CI は `mypage/**` の変更を上記の検証に分類する。運用 credentials は検証に不要。
