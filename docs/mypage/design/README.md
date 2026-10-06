# MyPage Approved Visual Reference

Status: **Approved**  
Approved date: **2026-10-03**

このディレクトリは、マイページPhase 1 UIの視覚・インタラクション参照を保持する。

## Source of Truth

- **画面・機能要件 / UX semantics**: Notion `01｜画面・機能仕様`
- **API wire contract**: GitHub `docs/mypage/openapi.yaml`（API契約PRで追加）
- **Visual / interaction reference**: このディレクトリ
- **実装コード・実装済み状態**: GitHub

Visual Reference内のサンプルデータや仮の技術実装を、業務仕様・API仕様として扱わない。
プロダクト動作はNotion Current CanonとGitHub API contract / 現行ドメインモデルを優先する。

## Approved files

### `approved-prototype.dc.html`

確定したメインprototype。

SHA-256（元のApproved export）:

`5a1413c1c3ba88d3297e540699d765f5a4a40a52b935dc5ef4b5a3b456bfbc68`

### `verification.dc.html`

レスポンシブ・状態別の視覚確認用gallery。

SHA-256（元のApproved export）:

`317f5350a4c2c8f9df745a902bec292577783cdd683bfbb3f7235141c030dd28`

## Required visual coverage

最低限、以下のviewportを確認する。

- 320px
- 390px
- 768px
- 1024px
- 1440px

主な状態:

- 作業中
- 休憩中
- 未入室
- 0時間
- Study Space未登録
- 長い作業名
- 終了予定超過
- 初回loading
- 初回取得失敗
- 更新失敗（最後の成功data保持）
- YouTube再認証
- Googleログイン
- YouTubeチャンネル確認
- channel mapping競合
- アカウントパネル / 管理
- Privacy / Cookie / Contact / データ削除導線

## Important semantics

### 休憩

休憩専用の作業内容は存在しない。

- `workName` を休憩中も保持する
- 作業名のlabelは「いまの作業」
- 状態は「休憩中」
- 時刻は「休憩開始」「休憩終了予定」として表示する

### Account

ヘッダーのavatarからaccount panelを開く。
Mobileではbottom sheet、広いviewportではavatar付近のpopoverとして扱う。

### Accessibility

prototypeに含まれる以下の性質を実装でも維持する。

- keyboard focus
- modal / dialog focus trap
- Escapeで閉じる
- status / alert / aria-live
- `prefers-reduced-motion`
- 44px程度のinteractive target
- 色だけに依存しない状態表現

## Not approved as final content

以下の文面はVisualの配置・導線確認用Draftであり、公開文面の承認を意味しない。

- Privacy Policy本文
- 利用規約本文
- Contact受付文面
- データ削除依頼の最終運用文面

公開時はNotionのPrivacy / 利用規約 Current CanonとGitHub実装を照合する。

## Implementation rule

このHTMLをproduction codeへそのままコピーしない。

Codex / 実装者は、

1. 現行 `dev` のdomain modelを確認する
2. Notion Current Canonを確認する
3. GitHubのAPI contractを確認する
4. 現行React / TypeScript構成へ再実装する
5. このApproved Visualと主要状態を視覚QAする

という順で扱う。

> `.dc.html` はClaude Design由来の参照sourceであり、実行runtime自体をproduct dependencyにはしない。

## Current Canon reconciliation

追跡済みの参照HTMLはfeature/mypageのbyte列を変更せず取り込んだ。最新仕様により背景はflat cream、通常cardは常時shadowなし、状態・集計・YouTube確認は最新contractを優先する。実装側は合成fixtureで320/390/768/1024/1440pxとkeyboardを実browserで検証する。

追跡済みfeature exportのSHA-256は approved-prototype: `cb7e6c373ee38a58aafeb2c3bc826222670d0ba1f3bfb70ad6fc66f0193c5108`、verification: `0bf10a1c8c624a9c8fbf269d0191ba0004ce7950f276b11c84dcb4fbb9e6c768`。上記historical Approved hashとは一致しないため、追跡済みHTMLだけを元Approved exportのbyte正本とは扱わない。

## Supplied Approved runtime comparison (2026-10-06)

ユーザーが単体で提供した `マイページ.dc.html`（74,955 bytes）と `マイページ デザイン案.dc.html`（7,067 bytes）は、Libraryの全文text readを連結してbyte数を確認し、上記historical Approved SHA-256の双方と一致した。併せて `support.js`（69,150 bytes、SHA-256 `8fe7df74405f3c55f49b7249c74ea1397e65d07dea2b1bd3b4a489bec2e28cbe`）を受領した。提供ファイルはconsumer-localの確認用に保持し、公開repositoryへ追加しない。

原本を隔離したlocalhostのChromiumで実行し、PC 1440×960 / Mobile 390×960の作業中・チャンネル確認・初回取得失敗を現行React画面と比較した。原本runtimeのReact 18.3.1 / React DOM 18.3.1 / Babel standalone 7.29.0はnpm package内の配布物をlocalで解決し、browserから外部へのrequestは遮断する。原本に秘密情報の既知patternや実認証呼出しは見つからず、prototypeのstate切替とmock dataだけを確認に使用する。

原本に合わせてカード・時間軸の余白、数字と単位の階層、補助指標の配置、セージ色、チャンネル確認のidentity中心の配置を調整した。Notion Current Canonで固定されたheader / greeting / 見出し・flat背景・shadowなし・snapshot時刻・本人確認の文言とflowは原本より優先する。全体初回取得失敗は単一のerror card、partial取得失敗は利用可能sectionを維持する。loadingは同じ3領域のSkeletonを表示する。

外部font通信は遮断し、両画面はlocal fallback fontで比較している。原本と製品fixtureのsample値・時刻は異なるため、データ差をpixel不一致の根拠にはしない。完全pixel parityや実OAuth・Hosting E2E・公開readyの確認は含めない。
