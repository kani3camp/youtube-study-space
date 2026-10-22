# MyPage visual reference and acceptance

Phase 1のデザイン方針は[Notion 01｜画面・機能仕様](https://app.notion.com/p/3ec357a8d0ce81e293d1d4cb739d3063)、現在の表示は[React component](../../../mypage/src/features/mypage/view.tsx)と[stylesheet](../../../mypage/src/style.css)を参照する。デザイン承認は公開文面・実認証・一般公開の承認とは別に扱う。

## 表示の原則

- flat cream背景と、明るいsurface・薄いborderを使う。通常cardに常設shadowを付けない。
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

表示単体は[visual entrypoint](../../../mypage/src/visual.tsx)と[visual QA](../../../mypage/scripts/visual-qa.py)、認証・routeを含む操作は[runtime QA](../../../mypage/scripts/runtime-qa.py)を使用する。これらはdev専用の合成検証で、productionの入口に含めない。

```sh
cd mypage
pnpm dev --host 127.0.0.1
# 別shell、Playwright/Chromiumを利用できる環境
python -m pip install -r visual-requirements.txt
python scripts/visual-qa.py --output /tmp/mypage-visual-qa
python scripts/runtime-qa.py --output /tmp/mypage-runtime-qa
python scripts/font-qa.py --output /tmp/mypage-font-qa
```

外部requestを遮断し、撮影前にfont loadを待つ。実ユーザーdataをpublic artifactへ保存しない。合成fixtureの成功やprototypeとの比較を、実OAuth・Hosting・公開Readyの証拠へ読み替えない。

承認原本のhash・比較記録と旧HTML exportは[Archive](../archive/README.md)に保持する。旧exportはApproved原本とbyteが一致せず、欠落した`support.js`に依存するため、現行の実行可能な視覚基準として使わない。
