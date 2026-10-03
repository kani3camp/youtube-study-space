# mypage

React + TypeScript + Vite で構成された `mypage` フロントエンドです。
コード品質チェックは ESLint ではなく Biome を使用します。

## Interactive design mock

Public Standard v1 の画面設計をレビューするときは、mock mode で起動します。

```bash
cd mypage
VITE_USE_MOCK=true pnpm dev
```

`http://localhost:18081/` を開くと、通常のAPIデータではなくレビュー用のインタラクティブモックを表示します。

モック上部の「レビュー操作」から、次の状態を切り替えられます。

- 作業中
- 休憩中
- 未入室
- Study Space未登録
- YouTube連携切れ
- APIエラー
- Loading / Skeleton

7日グラフは各日をクリック・フォーカスして正確な作業時間を確認できます。
YouTubeライブ、再連携、ログアウト、Privacy Policy、利用規約、Cookie設定、問い合わせの各導線もモック上で操作できます。

`VITE_USE_MOCK` を指定しない通常起動では、既存の認証・APIフローと `MyPageView` をそのまま使用します。

## Scripts

- `pnpm dev`: 開発サーバー起動
- `pnpm build`: TypeScript ビルド + Vite ビルド
- `pnpm preview`: ビルド成果物のプレビュー
- `pnpm typecheck`: TypeScript 型チェック
- `pnpm lint`: Biome lint
- `pnpm format`: Biome format（書き込みあり）
- `pnpm check`: Biome check
- `pnpm check:fix`: Biome check（書き込みあり）
