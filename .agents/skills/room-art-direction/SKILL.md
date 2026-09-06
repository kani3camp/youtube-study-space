---
name: room-art-direction
description: YouTube Study Space のルーム背景画像について、画像生成・プロンプト作成・レビュー・方向性比較を行うときに使う。描画スタイル、Look、題材・レイアウトを分離し、再利用可能なアートディレクションと配色・光・材質感のLookを適用する。
---

# Room Art Direction

YouTube Study Space のルーム背景画像に、再利用可能なアートディレクションを適用するためのスキル。

## 目的

- 一度合意した「描き方」を、題材が変わっても再現する。
- 世界観、モチーフ、建築、レイアウトを画風と混同しない。
- 配色・ライティング・材質感の好みを、Directionそのものへ不用意に焼き込まない。
- 画像生成モデル固有の偶然の癖を、プロジェクトのデザインルールとして固定しない。
- 後続の新しいDirectionやLookを再利用可能な形で資産化する。

## 入力として確認するもの

タスクから次を読み取る。

1. 使用するアートディレクション。
2. 使用するLook。未指定なら、CLIでは `auto`、手動の画像生成では目的に応じて選ぶ。
3. 今回の題材、場所、世界観。未指定なら自由に設計してよい。
4. 画像用途上の制約。例: 16:9、座席カードを後置きする、人物なし、文字なし。
5. 今回だけの必須要素・禁止要素。

Direction、Look、題材は別の入力として扱う。

## 現在のアートディレクション

| ID | Reference | 要約 |
| --- | --- | --- |
| Direction A | `references/direction-a-clean-vivid-digital.md` | CG / ゲーム環境系。立体・素材・ライティングを強く使い、わくわくする高品質スタイライズド環境アート |
| Direction B | `references/direction-b-full-scene-anime.md` | 全景アニメ系。現代アニメの自然さ、劇場アニメの光と空気、クリーンなセル表現を統合 |
| Direction C | `references/direction-c-clean-abstract-graphic.md` | 抽象・グラフィック系。形・色面・構成を主役にし、明るいパステル配色と非写実的な物体表現を重視 |
| Direction D | `references/direction-d-clean-cozy-digital.md` | クリーン・コージーなデジタルイラスト系。整理された具体形、柔らかな陰影、簡略化した素材、調和した色設計を重視 |

管理人が Direction A / CG・レンダー系、Direction B / アニメ系、Direction C / 抽象・グラフィック系、Direction D / クリーン・コージーなデジタルイラスト系などを指定した場合は、対応する reference を読む。

方向性が明示されていない場合、既存 Direction を勝手にデフォルト扱いしない。必要なら複数方向を比較できる形で提示する。

> [!NOTE]
> Direction D の品質基準は ChatGPT Chat / GPT-5.6 Sol / High での生成を主とする。Codex / Work / 軽量モデル固有の失敗を補正するための長いnegativeを canonical Direction へ積み増さない。別経路の生成結果はモデル比較として切り分ける。


## 現在のLook profiles

Look は、Direction の描画文法を保ったまま、**配色・ライティングのムード・材質感の強調方向**を切り替えるレイヤー。

| ID | Runtime source | 要約 |
| --- | --- | --- |
| `indigo-violet-fantasy` | `tools/room-image-prompt/data/look_indigo_violet_fantasy.txt` | 青・インディゴ・紫・シアンを軸にした幻想的な光と色。暗部も色を保ち、発光感を出す |
| `airy-garden` | `tools/room-image-prompt/data/look_airy_garden.txt` | 空色・緑・アイボリー/ベージュ・温かい木色を軸にした、明るく爽やかな庭園系 |
| `coral-aqua-glow` | `tools/room-image-prompt/data/look_coral_aqua_glow.txt` | コーラル/ピーチ/アンバーとアクア/ターコイズの暖冷対比。水がある場合は透明感を強調 |
| `crystal-lucent` | `tools/room-image-prompt/data/look_crystal_lucent.txt` | 淡いシアン・ラベンダー・乳白色と、ガラス/クリスタル/半透明の軽やかな材質感 |

Look のruntime sourceは上記 `data/look_*.txt` とし、Directionのcanonical Markdownとは分けて管理する。

> [!IMPORTANT]
> Look は指定された時間帯・天候を上書きしない。例えば `indigo-violet-fantasy` を選んでも昼を夜へ変えず、昼ならcool shadowやcolored accentとして表現する。
>
> Look はDirectionも上書きしない。Direction Bならアニメとして、Direction Cなら抽象グラフィックとして、Direction Dなら簡略化した2Dデジタルイラストとして、同じLookをそれぞれの描画言語へ翻訳する。

## ワークフロー

### 1. 用途制約・Direction・Look・題材を分ける

まず、次を別々に整理する。

**用途制約**
- アスペクト比
- 座席数
- UI を重ねる領域
- 人物・文字の有無
- 配信画面としての視認性

**Direction**
- 線や輪郭の扱い
- 面・形の整理方法
- 陰影の作り方
- 2D / 3D の立体感
- 素材をどの程度リアル/簡略化して描くか
- 情報密度
- エッジと仕上げ

**Look**
- パレットの色相傾向
- 暖色 / 寒色の関係
- ライティングのムード
- 水・ガラス・クリスタルなどの材質感をどの程度強調するか
- 透明感・発光感・爽やかさなどの視覚的な印象

`tools/room-image-prompt/data/prompt_template.txt` のような既存資料を参照する場合も、この区別を維持する。
用途制約、Theme、Direction、Lookのどれかを、別レイヤーへ安易に押し込まない。

> [!IMPORTANT]
> Direction の canonical source は `references/direction-*.md` である。CLI用の `tools/room-image-prompt/data/style_direction_*.generated.txt` は各referenceの **`## Prompt guidance` セクションから自動生成**されるため、生成物を直接編集しない。
> Direction を変更したら `cd tools/room-image-prompt && go generate ./data` を実行し、生成物も同じ変更に含める。CIはcanonical Markdownと生成物の不一致を拒否する。
> CLIでは `-style direction-a` / `direction-b` / `direction-c` / `direction-d` で生成済みDirectionを利用できる。未指定時だけ後方互換のため `legacy` を使う。
> Look は `-look <name>`、任意Lookは `-look-file <path>` で指定する。Look未指定時は `auto` としてbundled Lookから1つ選ぶ。`-look none` で無効化できる。

### 2. Direction の不変条件を読む

選択された reference の以下を抽出する。

- Core
- Prefer
- Avoid
- Non-goals

特に Non-goals を重視する。
過去にうまくいった画像の「何が描かれていたか」を、次回の必須モチーフに昇格させない。


### 2.5. Look を独立して適用する

Look が指定されている場合は、対応する `tools/room-image-prompt/data/look_*.txt` を読む。

Look から使うのは次のような要素。
- 色相と明度の関係
- 暖冷バランス
- ライティングのムード
- 水・ガラス・クリスタル等の材質感の強調方法

Lookから題材・建築・座席配置を逆算しない。例えば `crystal-lucent` だからといってクリスタル宮殿を必須にせず、選ばれたSceneの中で自然に成立する透明感として適用する。

### 3. 今回の題材を独立して設計する

Direction を保ったまま、題材・建築・地形・時代・世界観・レイアウトは今回の目的に合わせて自由に設計する。

例:
- 前回が浮遊都市だったからといって、次回も空、水、白い建築、円形足場を使う必要はない。
- 地下空間、雪原、都市屋上、宇宙船、学校、温室、商業施設などへ大胆に変更してよい。

「同じ画風」と「同じ世界観」を同義にしない。

### 4. 画像生成またはプロンプト化する

画像生成を依頼された場合は、用途制約・Theme・選択Direction・Lookを統合して生成する。

テキストプロンプトを依頼された場合は、特定モデルへの依存を抑えた形で次の順に記述する。

1. 用途と構図
2. 今回の題材
3. 選択 Direction の描画ルール
4. 選択 Look の配色・光・材質感
5. 必須要素
6. Avoid
7. 「過去例のモチーフを固定しない」旨

### 5. 比較画像では比較条件を固定する

複数 Direction を比較するときは、Lookを含めて描画スタイル以外をできるだけ同一にする。
複数 Look を比較するときは、Direction・題材・構図・時間帯・天気を同一にする。

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

厳密なスタイル比較では、まず `references/style-comparison-benchmark.md` の**text-locked fixture**を使い、題材・構造アンカー・時間帯・天気・光源条件を同一テキストで固定する。画像referenceはgeometryを揃えやすい反面、そのreference固有の材質・陰影・3D感がDirectionへ混入することがあるため、Style fidelityの一次判定にはしない。

同一geometryでの変換能力も確認したい場合は、A/B/C/Dのどれにも属さないneutral clay / blockout画像をsecondary benchmarkとして使う。完成したA/B/C/D画像をreferenceにしない。reference使用時に画風が弱まった場合は、失敗をDirection定義へそのまま取り込まず、**reference bias / geometry-style trade-off**として分離評価する。

生成ごとの空間設計差は Style fidelity とは別に **geometry drift**、時間帯・天気・光源のズレは **condition drift** として記録する。

### 6. 結果をレビューする

レビューでは最低でも次の3軸を分けて評価する。

**Style fidelity**
- Direction の線、陰影、立体感、素材の抽象度、仕上げを守っているか。

**Look fidelity**
- 選択Lookの色関係、ライティングのムード、透明感や材質感が出ているか。
- Lookが時間帯・天候・Directionを上書きしていないか。

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

## 新しい Look を追加するとき

1. `tools/room-image-prompt/data/look_<name>.txt` を追加する。
2. 色・光・材質感のルールだけを書き、特定の世界・家具・レイアウトを必須にしない。
3. 指定された時間帯・天候を上書きしないことを明記する。
4. Direction A/B/C/D の少なくとも複数で適用可能な表現にする。
5. `internal/look` の bundled list とテストへ追加する。
6. `go test ./...` でCLI合成とLook解決を確認する。

既存DirectionへLook固有のパレットを焼き込んで平均化しない。

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
