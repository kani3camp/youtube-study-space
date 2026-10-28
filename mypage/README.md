# MyPage

React / TypeScript / Vite の独立したfrontend package。認証・取得・同意のclient/runtimeとVitestを持つ。実装契約は[MyPage contract](../docs/mypage/README.md)、視覚確認は[design acceptance](../docs/mypage/design/README.md)、配備・実試験の最新状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。[release手順](../docs/mypage/release/README.md)で設定preflight・実E2E・公開の完了条件を確認する。

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

CI は `mypage/**` の変更を上記の検証に分類する。運用 credentials は検証に不要。

## 依存関係とビルドのセキュリティ検査

Nodeはルートの`.node-version` / `.nvmrc`を使う。CIは開発依存も含む`pnpm audit --audit-level=high`でHigh / Criticalを拒否する。Go側は`govulncheck -test ./...`、MyPageの実ビルドイメージはTrivyでOS / libraryのHigh / Criticalを検査し、修正版のない指摘も自動では除外しない。検査失敗や脆弱性DB取得失敗を成功として扱わない。

DependabotはMyPage frontend / Go / Dockerを統合ブランチ向けの日次更新対象にする。設定はリポジトリのdefault branchに到達してから有効になるため、このPRのCI成功だけで日次運用の開始とは扱わない。統合PR #1236の完了時には更新先ブランチも見直す。CodeQL成功、依存監査成功、実環境Security Gateの完了はそれぞれ別の検証結果として扱う。
