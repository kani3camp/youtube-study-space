# マイページ デザインシステム

この文書は、オンライン作業部屋のマイページUIに限定した**実装用UI仕様**です。

Notion「01｜画面・機能仕様」の第1〜9節は主に要件・プロダクト方針を定義し、この文書と同ページ第10節は「どう表示・配置・操作させるか」を定義します。Phase 1 Public Standardについては、本書の「Phase 1 Public Standard 実装仕様」に記載した固定値・挙動を実装正本とします。

広報物や配信画面のデザインをそのまま移植するのではなく、Claude Designで検討したマイページ案から、実装で再利用できるデザイン言語を抽出しています。機能追加の有無にかかわらず、今後のマイページ画面は原則としてこの指針に沿って拡張します。

## 目指す体験

マイページは「管理画面」ではなく、自分の作業の現在地と積み重ねを静かに確認する個人用スペースとして扱います。

優先する印象:

- やわらかい
- 落ち着いている
- 大人が日常的に使えるかわいさ
- 数字の積み上げが気持ちよく見える
- 情報量が増えても圧迫感がない
- 汎用SaaSダッシュボードに見えすぎない

避ける印象:

- 強い競争・ランキング感
- ゲーミング、ネオン、サイバー
- ビジネス管理画面らしい硬さ
- カードを均等に大量配置するだけのダッシュボード
- 過度に子ども向け・ファンシー
- 重要度の異なる情報を同じ強さで並べること

## 情報の優先順位

デザイン上の強さは、原則として次の順にします。

1. 現在の作業状態
2. 今日の作業時間
3. 累計作業時間
4. 最近の活動・作業履歴
5. YouTubeアカウント情報
6. 設定・ログアウト等の管理操作

特に「現在の作業状態」は通常のサマリーカードと同列にせず、ページ上で一段強く見せます。

## タイポグラフィ

### Primary font

`M PLUS Rounded 1c`

```css
font-family: "M PLUS Rounded 1c", "Hiragino Maru Gothic ProN", "Yu Gothic", sans-serif;
```

Google Fontsでは `400`, `500`, `700`, `800` を読み込みます。

### Weight

- 400: 長い説明文
- 500: 補助テキスト
- 700: ラベル、ボタン、短い強調
- 800: ページタイトル、状態名、主要な数値

### 数値

作業時間・開始時刻・終了予定時刻には必ず `font-variant-numeric: tabular-nums` を使います。

## カラー

マイページでは、暖色寄りのニュートラルカラーを面積の大部分に使います。ブランド色を画面全体に敷くのではなく、状態・CTA・小さなアクセントに限定します。

### Core tokens

| Token | Value | 用途 |
| --- | --- | --- |
| `--color-bg` | `oklch(97% 0.008 75)` | ページ背景 |
| `--color-surface` | `oklch(99% 0.004 80)` | カード・情報面 |
| `--color-surface-soft` | `oklch(95% 0.01 70)` | 補助面 |
| `--color-border` | `oklch(89% 0.012 70)` | 境界線 |
| `--color-text` | `oklch(24% 0.02 55)` | 本文・見出し |
| `--color-text-muted` | `oklch(48% 0.018 55)` | 補助テキスト |
| `--color-text-subtle` | `oklch(64% 0.014 55)` | ラベル・補助情報 |
| `--color-accent` | `oklch(64% 0.1 55)` | 主アクセント |
| `--color-accent-soft` | `oklch(93% 0.035 55)` | アクセントの淡い面 |
| `--color-break` | `oklch(62% 0.08 170)` | 休憩状態 |
| `--color-break-soft` | `oklch(94% 0.025 170)` | 休憩状態の淡い面 |
| `--color-danger` | `oklch(50% 0.16 30)` | エラー |

アクセント色は大面積に使いすぎず、状態ドット、バッジ、CTA、現在作業カードの小さな強調に使います。

## レイアウト

### 基本

- Mobile / PCを同格の主要ターゲットとして扱う
- 情報の優先順位と主要機能は共通にしつつ、レイアウト・余白・情報密度・操作導線は画面幅ごとに最適化する
- Compact: `0〜767px`、1カラム
- Medium: `768〜1099px`、1カラム
- Wide: `1100px以上`、Main + Side rail の2カラム
- Wideでは Main に「現在の作業 → 直近7日」、Side rail に「作業サマリー → YouTubeアカウント」を置く
- Wideのgridは `grid-template-columns: minmax(0, 2fr) minmax(320px, 1fr)`
- Wideのカラムgapは `24px`
- コンテンツ全体の `max-width` は `1120px`
- Compact / Mediumのセクション間gapは `18px`
- 横幅を埋めること自体を目的にせず、情報のまとまりと主従関係を優先する

Phase 0A〜Phase 1では常設サイドバーを導入しません。機能が増えた場合も、まずヘッダー・セクション・軽量なナビゲーションで解決できないか検討します。

## Shape / spacing

### Radius

- 情報カード: `18px`
- 主役カード: `22px`
- ボタン: `12px`
- バッジ: pill (`999px`)
- アバター: circle

極端に大きな角丸をすべてに適用せず、主役カードだけ少し柔らかくします。

### Shadow

Phase 1 Public Standardのcontent cardには `box-shadow` を付けません。カードの分離は背景色と1px borderで行います。

## コンポーネント原則

### Current work card

マイページの主役です。

- 現在状態を最初に認識できる
- 作業中 / 休憩中 / 未入室を色だけに依存せず、文言でも表す
- 作業内容は省略せず全文を表示する
- 席番号はバッジ等の補助情報として扱う
- 開始時刻・終了予定などは一段弱い情報として整理する
- 作業中と休憩中で状態色を変えてよい

### Summary

今日・今週・累計は、独立カードを3枚並べるのではなく、1つのSummary領域に3指標としてまとめます。

- 順序は「今日 / 今週 / 累計」
- ラベルより数値を主役にする
- 数値は ExtraBold を基本とする
- Mobileでも原則3列を維持し、320px幅まで横スクロールなしで収める
- 目標達成率、前週比、順位はPhase 1 Public Standardでは表示しない
- 0時間は正常値として扱う
- 現在作業カードより視覚的に弱くする

### Account card

YouTubeアカウント情報は確認用途であり、ページの主役ではありません。

- アバターは `48px × 48px`
- 表示名と「連携済み」statusを表示する
- channel IDはPhase 1 Public Standardでは表示しない
- 表示名は最大2行とし、2行を超える場合だけellipsisを使用する

### Buttons / CTA

Primary CTAは「そのdecision surfaceで最も推奨する1つの主操作」です。同一surface内にPrimary CTAを2つ置きません。

Primary button:
- `min-height: 44px`
- Compactでは `width: 100%`
- Medium / Wideでは `width: auto`
- 左右padding `16px`
- `border-radius: 12px`
- accent背景 + 白文字
- font-weight `700`

Secondary button:
- `min-height: 44px`
- surface背景 + 1px border
- 左右padding `16px`
- `border-radius: 12px`
- 通常本文色

Text action:
- 背景なし
- underlineまたは明確なhover / focus表現
- interactive areaの最小高さ `44px`

状態別:
- 未ログイン: 「Googleで続ける」= Primary
- YouTube未連携: 「YouTubeチャンネルを連携」= Primary
- チャンネル確認: 「このチャンネルを連携」= Primary、「別のアカウント / チャンネルでやり直す」= Secondary
- 未入室: 「YouTubeライブを開く」= Primary
- 作業中 / 休憩中: 「YouTubeライブを開く」= Secondary
- 初回取得失敗: 「再読み込み」= Primary
- Account: 「再連携」= Secondary、「ログアウト」= Text action

危険操作以外で赤を使いません。

## 状態表現

状態は色だけで伝えません。

- 作業中: warm accent + 「作業中」
- 休憩中: green/teal accent + 「休憩中」
- 未入室: neutral + 「未入室」
- エラー: danger + 説明文

## レスポンシブ

### Mobile

- 1カラム
- 左右padding `16px`
- 主要数値は本書の固定 `clamp()` 指定に従う
- 情報の並び順を優先し、無理な横並びをしない

### Desktop / tablet

- Mobileと同じ情報優先順位を保ちつつ、レイアウトはDesktop向けに独立して最適化する
- Mediumでは無理に2カラム化しない
- Wideでは Main + Side rail を使い、Mainに現在の作業と7日グラフ、Side railにSummaryとYouTubeアカウントを置く
- 横幅を埋めるためだけの多カラム化は禁止
- 代表的なMobile幅とDesktop幅の双方で、作業中 / 休憩中 / 未入室 / エラー / 連携切れの主要状態を視覚QAする
- 320px幅でも主要情報・CTAが横スクロールなしで利用できることを最低条件とする

## Data presentation

- 作業時間は分単位を基本とし、秒単位の常時カウントアップは行わない
- 「今日」「今週」「直近7日」の集計境界はJST
- 今週は月曜00:00 JST開始
- 開始 / 終了予定時刻もJSTで表示し、UI上でJSTであることを一度は明示する

## Accessibility

- 状態は色だけで伝えず文言を併用する
- interactive要素のタップ領域は最小 `44 × 44px`
- キーボードfocus indicatorを消さない
- グラフの値確認をhoverだけに依存させず、focus / tapでも確認可能にする
- `prefers-reduced-motion` を尊重する
- 画面拡大 / 文字拡大でも主要情報とCTAを欠落させない

## Copy tone

- 短く、静かで自然な日本語
- 達成を過度に煽らない
- 未利用・未達をネガティブに扱わない
- 「自分の積み重ねを確認する」体験を壊す競争的表現を避ける

## Release Phaseとの対応

リリース単位はNotion「02｜リリース・YouTube導線」で定義されたPhaseのみを使用します。UI仕様側で別系統の製品versionを定義しません。

| Phase | UI surface | 適用 |
| --- | --- | --- |
| Phase 0A Silent Production | 現在の作業、今日、累計、YouTubeチャンネル、ログアウト | 本書の共通token / typography / responsive / accessibilityを適用する。Phase 1専用セクションは表示しない |
| Phase 0B Private Canary | 原則Phase 0Aと同じ | 新機能追加より、Mobile / Desktop、複数Googleアカウント、Brand Account、未登録、scope拒否、revoke、再連携等の状態差を検証する |
| Phase 1 Public Standard | Phase 0B + 今週、直近7日、正式Account / Footer / Consent導線 | 下記「Phase 1 Public Standard 実装仕様」を全面適用する |
| Phase 2 Premium Launch | Phase 1を基礎に期間・詳細度等を拡張 | 別ホームを作らずPhase 1の情報階層を拡張する |

Phase 0A / 0Bで情報量が少なくても、Phase 1以降の空カードや無効ナビゲーションを先置きして画面を埋めません。


## Phase 1 Public Standard 実装仕様

### Page shell

- Compact: padding `24px 16px 40px`
- Medium: padding `32px 24px 48px`
- Wide: padding `40px 32px 56px`
- content max-width: `1120px`
- content: viewport中央寄せ
- Header / Footerはcontent全幅
- Side railはstickyにしない

### Card

通常card:
- `border: 1px solid var(--color-border)`
- `border-radius: 18px`
- Compact / Medium padding: `20px`
- Wide padding: `22px`
- `box-shadow: none`
- 固定height / max-heightを指定しない

Current work card:
- `border-radius: 22px`
- Compact / Medium padding: `22px`
- Wide padding: `26px`
- `box-shadow: none`

### Header

- eyebrow: 「オンライン作業部屋」
- h1: 「マイページ」
- avatar / account actionは置かない
- h1 Compact: `30px`
- h1 Medium / Wide: `40px`
- font-weight: `800`

### Current work

表示順:
1. 「現在の作業」label + 席badge
2. 状態
3. 作業名
4. 開始 / 終了予定
5. CTA

作業名:
- 空文字: 「作業内容未設定」
- ellipsis禁止
- `line-clamp` 禁止
- `max-height` 禁止
- `white-space: normal`
- `overflow-wrap: anywhere`
- `word-break: break-word`
- 全文表示し、必要な行数だけcardを縦に伸ばす
- Compact: `font-size: 28px; line-height: 1.35; font-weight: 800`
- Medium: `32px / 1.35 / 800`
- Wide: `36px / 1.30 / 800`

workNameには既存入力系で文字数上限がないため、マイページ独自の文字数上限や切り詰めを追加しません。

meta:
- 開始 / 終了予定を `repeat(2, minmax(0, 1fr))`
- `HH:mm` 形式、JST
- 「日本時間（JST）」をcard内に1回表示
- 秒単位のカウントアップなし

### Summary

- 1枚のcardに「今日 / 今週 / 累計」
- `grid-template-columns: repeat(3, minmax(0, 1fr))`
- 列間に1px separator
- label: `12px; font-weight: 700`
- value: `font-size: clamp(18px, 5vw, 28px); font-weight: 800; line-height: 1.1; white-space: nowrap`
- 表示形式: `0m`, `25m`, `1h 05m`, `123h 45m`
- 0は `0m`
- 前週比 / 達成率 / 目標 / 順位を置かない

### Recent 7 days chart

- 7本固定
- `grid-template-columns: repeat(7, 1fr)`
- gap: `8px`
- plot height: Compact / Medium `140px`、Wide `160px`
- 期間中の最大値を100%として棒高を算出
- 全日0なら全棒0
- 0の日はdata value 0のまま、baseline上に2px neutral markだけ描画
- 今日: accent
- 過去6日: neutral
- x label: `M/D`
- 今日の列には追加で「今日」
- 各data pointをkeyboard focus可能にする
- hover / focus / tapで `M月D日 1h 05m` tooltip
- 横スクロール禁止
- Y軸 / 凡例 / 目標線 / 前週比較 / 期間切替なし

### Account

- avatar: `48px × 48px`
- 表示名 + 「連携済み」
- channel ID非表示
- 表示名は2行まで。3行目以降だけellipsis
- action order: 「再連携」→「ログアウト」
- 再連携: Secondary
- ログアウト: Text action

### Footer

表示:
- Privacy Policy
- 利用規約
- Cookie設定
- 問い合わせ / データ削除依頼

横幅不足時はwrapし、horizontal scrollを発生させません。

### Loading / refresh / error

Initial loading:
- 最終画面と同じ主要領域のSkeleton
- 全画面spinnerのみのloading禁止
- shimmerなし
- static surface-soft blockを使用

Refresh:
- request完了60秒後に次request
- request重複禁止
- hidden中はtimer停止
- visible復帰時に即時1回取得
- そのrequest完了後から60秒timer再開

Initial error:
- 0値で代替しない
- 「情報を取得できませんでした」
- Primary「再読み込み」

Refresh error after success:
- 最後の成功dataを保持
- 「最新情報に更新できませんでした。前回取得した情報を表示しています。」
- Text action「再試行」

### Accessibility / language / theme

- 日本語固定
- Light theme固定
- body base font-size: `16px`
- focus-visible: `3px` outline / `3px` offset
- interactive target: 最小 `44 × 44px`
- statusは文字併用必須
- `prefers-reduced-motion: reduce` ではanimation / transition無効
- 320px CSS viewportでhorizontal scrollなし
- browser 200% zoomで主要情報 / CTA欠落なし
