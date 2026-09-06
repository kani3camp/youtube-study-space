---
name: room-art-direction
description: YouTube Study Space のルーム背景画像について、画像生成・プロンプト作成・レビュー・方向性比較を行うときに使う。描画スタイルと題材・レイアウトを分離し、references に保存した再利用可能なアートディレクションを適用する。
---

# Room Art Direction

YouTube Study Space のルーム背景画像に、再利用可能なアートディレクションを適用するためのスキル。

## 目的

- 一度合意した「描き方」を、題材が変わっても再現する。
- 世界観、モチーフ、建築、レイアウトを画風と混同しない。
- 画像生成モデル固有の偶然の癖を、プロジェクトのデザインルールとして固定しない。
- 後続の新しい方向性を `references/` に追加できる形で資産化する。

## 入力として確認するもの

タスクから次を読み取る。

1. 使用するアートディレクション。
2. 今回の題材、場所、世界観。未指定なら自由に設計してよい。
3. 画像用途上の制約。例: 16:9、座席カードを後置きする、人物なし、文字なし。
4. 今回だけの必須要素・禁止要素。

アートディレクションと題材は別の入力として扱う。

## 現在のアートディレクション

| ID | Reference | 要約 |
| --- | --- | --- |
| Direction A | `references/direction-a-clean-vivid-digital.md` | CG / ゲーム環境系。立体・素材・ライティングを強く使い、わくわくする高品質スタイライズド環境アート |
| Direction B | `references/direction-b-full-scene-anime.md` | 全景アニメ系。現代アニメの自然さ、劇場アニメの光と空気、クリーンなセル表現を統合 |
| Direction C | `references/direction-c-clean-abstract-graphic.md` | 抽象・グラフィック系。形・色面・構成を主役にし、明るいパステル配色と非写実的な物体表現を重視 |

管理人が Direction A / CG・レンダー系、Direction B / アニメ系、Direction C / 抽象・グラフィック系などを指定した場合は、対応する reference を読む。

方向性が明示されていない場合、既存 Direction を勝手にデフォルト扱いしない。必要なら複数方向を比較できる形で提示する。

## ワークフロー

### 1. 用途制約と画風を分ける

まず、次を別々に整理する。

**用途制約**
- アスペクト比
- 座席数
- UI を重ねる領域
- 人物・文字の有無
- 配信画面としての視認性

**アートディレクション**
- 線
- 面
- 色
- 光
- 質感
- 立体感
- 情報密度
- 仕上げ

`tools/room-image-prompt/data/prompt_template.txt` のような既存資料を参照する場合も、この区別を維持する。
その資料に含まれる用途制約は利用してよいが、特定の色・レンダリング・質感の指定を、選択中の Direction より優先しない。

> [!IMPORTANT]
> Direction の canonical source は `references/direction-*.md` である。CLI用の `tools/room-image-prompt/data/style_direction_*.generated.txt` は各referenceの **`## Prompt guidance` セクションから自動生成**されるため、生成物を直接編集しない。
> Direction を変更したら `cd tools/room-image-prompt && go generate ./data` を実行し、生成物も同じ変更に含める。CIはcanonical Markdownと生成物の不一致を拒否する。
> CLIでは `-style direction-a` / `direction-b` / `direction-c` で生成済みDirectionを利用できる。未指定時だけ後方互換のため `legacy` を使う。

### 2. Direction の不変条件を読む

選択された reference の以下を抽出する。

- Core
- Prefer
- Avoid
- Non-goals

特に Non-goals を重視する。
過去にうまくいった画像の「何が描かれていたか」を、次回の必須モチーフに昇格させない。

### 3. 今回の題材を独立して設計する

Direction を保ったまま、題材・建築・地形・時代・世界観・レイアウトは今回の目的に合わせて自由に設計する。

例:
- 前回が浮遊都市だったからといって、次回も空、水、白い建築、円形足場を使う必要はない。
- 地下空間、雪原、都市屋上、宇宙船、学校、温室、商業施設などへ大胆に変更してよい。

「同じ画風」と「同じ世界観」を同義にしない。

### 4. 画像生成またはプロンプト化する

画像生成を依頼された場合は、選択 Direction と用途制約を統合して生成する。

テキストプロンプトを依頼された場合は、特定モデルへの依存を抑えた形で次の順に記述する。

1. 用途と構図
2. 今回の題材
3. 選択 Direction の描画ルール
4. 必須要素
5. Avoid
6. 「過去例のモチーフを固定しない」旨

### 5. 比較画像では比較条件を固定する

複数 Direction を比較するときは、描画スタイル以外をできるだけ同一にする。

固定する対象:
- 題材と空間レイアウト
- カメラ位置・画角
- 時間帯
- 天気
- 光源条件
- 人物・小物の有無
- 用途制約

配色だけを比較するときは、形・質感・構図も固定する。
描画スタイルを比較するときも、時間帯や天気を不用意に変えない。

比較画像で「CG は夕方、アニメは昼、グラフィックは晴天」のように条件が混ざると、スタイル差を正しく評価できない。

厳密なスタイル比較では、ツールが画像参照に対応しているなら**A/B/Cのどれにも属さないスタイル中立のclay / blockout構図アンカーを先に作り、その同じ画像をreferenceとしてA/B/Cへ変換する**方法を優先する。完成したA/B/C画像を基準にすると、その画風が他Directionへ伝播するので使わない。カメラ、建築形状、座席位置、主要オブジェクト、時間帯、天気、光源方向を保持し、「変更してよいのは描画言語だけ」と明示する。

画像参照を使えない場合は、`references/style-comparison-benchmark.md` のように構造アンカーを明文化した比較条件を使う。生成ごとの空間設計差は Style fidelity とは別に **geometry drift** として記録する。

### 6. 結果をレビューする

レビューでは最低でも次の2軸を分けて評価する。

**Style fidelity**
- Direction の線、色、質感、光、仕上げを守っているか。

**Content/layout**
- 今回の空間設計や座席配置としてよいか。

「レイアウトが好みではない」ことを「画風が違う」と誤診しない。

## 検証方法

Direction を新規追加・大幅更新したときは、可能なら次の Subject Swap Test を行う。

1. 同一 Direction で、題材が大きく異なる2〜4案を作る。
2. 共通するモチーフを意図的に減らす。
3. それでも同じ描画方向として成立するか確認する。

例えば「空中庭園」で確立した Direction を検証するなら、地下施設、未来都市屋上、宇宙船、雪原基地などへ置き換える。

題材を変えると魅力が消える場合、その定義には画風ではなくモチーフが混入している可能性が高い。

## 新しい Direction を追加するとき

1. `references/direction-<id>-<name>.md` を追加する。
2. このファイルの「現在のアートディレクション」表へ追加する。
3. Core / Prefer / Avoid / Non-goals を必ず書く。
4. **`## Prompt guidance` を必ず1つだけ置き、CLIへ渡してよい簡潔なstyle fragmentだけを書く。** このセクションが実行用styleのcanonical sourceになる。
5. 過去生成物から抽出したモチーフと、実際の描画ルールを分離する。
6. Subject Swap Test で題材依存になっていないことを確認する。
7. `cd tools/room-image-prompt && go generate ./data` を実行し、生成された `style_direction_*.generated.txt` をコミットする。
8. `go test ./...` でcanonical sourceと生成物の同期を確認する。

既存 Direction の内容を、新しい Direction に合わせて平均化しない。管理人の複数の好みは複数の方向性として共存させる。
