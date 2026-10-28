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

## bfcache・利用制限の確認

HMR を含まない合成静的 build と Chromium で確認する。通常 / support × pending 有無の4ケースで `pageshow.persisted=true`、中断表示・再開始操作、古い応答・receipt・本人データの非復活を確認する。

```sh
cd mypage
pnpm exec vite build --config scripts/bfcache.vite.config.ts
python3 -m http.server 18082 --bind 127.0.0.1 --directory /tmp/mypage-bfcache-qa
# 別shell
python3 scripts/bfcache-qa.py
python3 scripts/service-access-qa.py
```

利用制限 / 削除中の確認は320 / 390 / 1440px、確認・完了・旧 session、bfcache 復帰、問い合わせ・トップ・fresh login・logout を対象とする。拒否後の自動 retry と旧データ復活を認めない。専用 build は production の入口に含めず配備しない。

## 仕様の優先順位

文言、本人確認、snapshot 時刻、状態と集計は最新 API 契約を優先する。背景・色・shadow は上記の表示原則に従う。提供された prototype と付属 runtime は公開 repository へ追加しない。sample 値や時刻の差を pixel 不一致の根拠にしない。
