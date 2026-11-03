# MyPage

React / TypeScript / Viteの独立したフロントエンドパッケージ。認証・データ取得・同意を扱うクライアントとランタイム、Vitestのテストを持つ。実装仕様は[MyPage仕様](../docs/mypage/README.md)、視覚確認は[表示の受入条件](../docs/mypage/design/README.md)、配備・実試験の最新状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。[配備手順](../docs/mypage/release/README.md)で、設定の事前検査・実環境のE2E・公開の完了条件を確認する。

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

CIは`mypage/**`の変更に上記の検証を適用する。運用認証情報は検証に不要である。

## 依存関係とビルドのセキュリティ検査

Nodeはルートの`.node-version` / `.nvmrc`を使う。CIは開発依存も含めて`pnpm audit --audit-level=high`を実行し、High / Criticalを拒否する。Go側は`govulncheck -test ./...`を使う。MyPageの実ビルドイメージはTrivyでOSとライブラリのHigh / Criticalを検査し、修正版のない指摘も自動除外しない。検査失敗や脆弱性DBの取得失敗を成功扱いにしない。

DependabotはMyPageフロントエンド・Go・Dockerを統合ブランチ向けの日次更新対象にする。設定はリポジトリの既定ブランチに入ってから有効になるため、このPRのCI成功だけで日次運用の開始とは扱わない。統合PR #1236の完了時には更新先ブランチも見直す。CodeQLの成功、依存監査の成功、実環境のセキュリティ確認の完了は、それぞれ別の検証結果として扱う。
