# MyPage

React / TypeScript / Vite の独立したfrontend package。認証・取得・同意のclient/runtimeと合成QAを持つ。実装契約は[MyPage contract](../docs/mypage/README.md)、視覚確認は[design acceptance](../docs/mypage/design/README.md)、配備・実試験の最新状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。[release手順](../docs/mypage/release/README.md)で設定preflight・実E2E・公開の完了条件を確認する。

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

CI は `mypage/**` の変更を上記の検証に分類する。運用 credentials は検証に不要。
