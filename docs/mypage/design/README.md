# MyPage visual reference and acceptance

Phase 1のデザイン方針は[Notion 01｜画面・機能仕様](https://app.notion.com/p/3ec357a8d0ce81e293d1d4cb739d3063)、現在の表示は[React component](../../../mypage/src/features/mypage/view.tsx)と[stylesheet](../../../mypage/src/style.css)を参照する。デザイン承認は公開文面・実認証・一般公開の承認とは別に扱う。

## 表示の原則

- クリーム背景に淡い光のgradientを重ね、明るいsurface・薄いborder・静的な柔らかなshadowを使う。作業中は暖色、休憩中はセージ系のshadow。常時animationは加えない。
- 現在の作業を主役とし、作業中は暖色、休憩中はセージ色で区別する。色だけに状態を依存させない。
- 休憩中も通常の`workName`を保持し、「いまの作業」「休憩開始」「休憩終了予定」を表示する。
- 未入室・未登録・正常0・不明・取得障害を区別する。不完全履歴の記録済み分は、確定合計と混同させない。
- Accountはヘッダーから開き、mobileではbottom sheet、広い幅ではpopoverとして表示する。
- keyboard focus、modalのfocus循環と背景inert、Escape、triggerへのfocus復帰、status/alert、reduced motion、約44pxの操作対象を維持する。
- 自己配信fontの由来・license・描画table・再現方法は[font README](../../../mypage/src/assets/fonts/README.md)を正本とする。

## 視覚・操作確認

代表幅は320 / 390 / 768 / 1024 / 1440px。これらは観測点であり、CSS breakpointの固定値ではない。

- 作業中・休憩中・未入室・未登録・0時間・長い作業名・終了予定超過。
- 初回loading、初回取得失敗、部分障害、更新失敗時の直前成功data保持。
- YouTubeログイン、チャンネル確認、期限切れや競合からのやり直し。
- Accountの開閉・管理・logout、画像欠落時の代替表示。
- Privacy / Terms / Cookie / Contact、利用制限、削除処理中。
- 直近7日のfocus/tap選択、正常0分の選択、グラフ外の説明、時刻ラベルの収まり。

表示単体は[visual entrypoint](../../../mypage/src/visual.tsx)で確認し、認証・routeを含む状態遷移は[runtime test](../../../mypage/src/runtime.test.ts)で検証する。visual entrypointはdev専用で、productionの入口に含めない。

```sh
cd mypage
pnpm dev --host 127.0.0.1
# 別shell、Playwright/Chromiumを利用できる環境
python -m pip install -r visual-requirements.txt
pnpm test
python scripts/font-qa.py --output /tmp/mypage-font-qa
```

外部requestを遮断し、撮影前にfont loadを待つ。実ユーザーdataをpublic artifactへ保存しない。合成fixtureの成功やprototypeとの比較を、実OAuth・Hosting・公開Readyの証拠へ読み替えない。

## bfcache・利用制限の確認

旧 session の遅延応答・復帰時の再検証・制限後の非復活は[runtime test](../../../mypage/src/runtime.test.ts)と[private memory test](../../../mypage/src/features/mypage/memory.test.ts)の代表ケースで検証する。

実 Hosting / Chromium の bfcache 復帰、利用制限 / 削除中の320 / 390 / 1440px表示、問い合わせ・fresh login・logout は[実経路E2E](../release/e2e.md)で確認する。HMRを実bfcacheの証拠にせず、拒否後の自動retryと旧データ復活を認めない。

## 仕様の優先順位

文言、本人確認、snapshot 時刻、状態と集計は最新 API 契約を優先する。背景・色・shadow は上記の表示原則に従う。提供された prototype と付属 runtime は公開 repository へ追加しない。sample 値や時刻の差を pixel 不一致の根拠にしない。
