# MyPage design / engineering docs

マイページのプロダクト仕様と実装仕様は、役割ごとに正本を分ける。

## Source of Truth

| 対象 | 正本 |
| --- | --- |
| プロダクト目的・Phase・画面/機能要件 | Notion Current Canon |
| 認証・アカウント方針 | Notion Current Canon |
| 作業時間・統計の意味 | Notion Current Canon |
| Privacy / Security | Notion Current Canon |
| API wire contract | [`openapi.yaml`](./openapi.yaml) |
| API設計意図 | [`API.md`](./API.md) |
| 実装・コード・テスト・CI | GitHub |
| Approved Visual Reference | GitHub（別PRで同期） |

同じrequest / response定義をNotionへ複製しない。

## Files

- [`openapi.yaml`](./openapi.yaml)
  - endpoint
  - request / response
  - HTTP status
  - error code
  - schema
- [`API.md`](./API.md)
  - APIの意味論
  - 認証境界
  - 統計定義との対応
  - 現行実装からの移行差分
- [`DESIGN.md`](./DESIGN.md)
  - 旧実装用UI文書。2026-10-03にApprovedとなった新しいVisual Referenceへの同期が必要
- [`mvp.md`](./mvp.md)
  - 旧MVPの混合仕様。API wire contractとしては使用しない

## Current Canon

- [マイページ｜構想・事業設計](https://app.notion.com/p/3873eac14a174f14881a18eb389f786c)
- [01｜画面・機能仕様](https://app.notion.com/p/3ec357a8d0ce81e293d1d4cb739d3063)
- [03｜データ・統計・計測](https://app.notion.com/p/3ec357a8d0ce81ac94eef9e6ac0c9964)
- [04｜認証・アカウント・Premium権限](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)

NotionとGitHubがずれている場合は暗黙にどちらかへ寄せず、「未同期」として扱ってから実装する。
