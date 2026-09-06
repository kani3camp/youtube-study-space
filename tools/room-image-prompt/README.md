# room-image-prompt

ルーム画像生成（画像生成モデル等）向けのプロンプトを組み立て、`output/` に保存する Go CLI です。候補テキストと共通テンプレートは `data/` に置き、`go:embed` でバイナリに同梱します。Direction A/B/C/D の正は `.agents/skills/room-art-direction/references/` にあり、CLI用style assetはそこから生成します。

## 必要環境

- Go 1.25.0 以上（リポジトリの `system/go.mod` に合わせています）

## ビルド・実行

```bash
cd tools/room-image-prompt
go build -o room-image-prompt ./cmd/room-image-prompt
./room-image-prompt -version
```

引数なしで、上から4テーマ行（各 `data/0N_*.txt` から1行を独立に乱数抽選）に加え、**座席数 10〜15** を1回一様乱数で決定します。さらに、色・光・材質感を担当する bundled **Look profile を1つ自動選択**します。`data/prompt_template.txt` の `{{STYLE}}` には Direction / style と Look を合成した visual guidance を挿入し、最後にテーマ条件を連結した UTF-8 テキストを **`output/prompt-<タイムスタンプ>.txt`** に書き込みます。あわせて**同じ本文をクリップボードへコピー**します（失敗しても終了コードは成功のままです）。

`-style direction-a` / `direction-b` / `direction-c` / `direction-d` で管理人のアートディレクションを選択できます。`-style-file <path>` を指定すると任意の UTF-8 テキストをスタイルとして注入できます。style 未指定時は後方互換のため `legacy` です。

Look は `-look <name>` で固定でき、`-look none` で無効化できます。未指定時は `auto` として bundled Look から1つ選びます。`-look-file <path>` では任意の Look guidance を注入できます。

- **標準エラー**: `出力: <ファイル名>` に続き、`クリップボードにコピーしました` または `コピーに失敗しました` を1行ずつ出します。Linux などで `xclip` / `xsel` が無い環境ではコピーが失敗し得ます。
- **標準出力**: **保存したファイルの絶対パスを1行**だけ出します（スクリプト向け）。

実行時は **`tools/room-image-prompt` をカレントディレクトリにした状態**で動かす想定です（省略時の出力先が `./output/` になるため）。

### テーマ候補ファイル（ステップ順）

| ファイル | 責務 |
|----------|------|
| `data/01_world.txt` | **Scene / World**。画像の舞台・環境を決める具体的な場所や世界。季節・天候は、そのSceneと不可分な場合だけ内包してよい |
| `data/02_time_of_day.txt` | **Time of Day**。純粋な時間帯だけを持つ |
| `data/03_workspace_type.txt` | **Workspace Type**。そのSceneの中に作る、実際の作業・学習・休憩空間の種類 |
| `data/04_seat_layout.txt` | **Seat Layout**。座席の空間的な配置・ゾーニング・動線だけを持つ |

座席数（10〜15）は専用ファイルを使わず、毎回一様乱数で1つ選びます。

#### 候補軸の設計原則

v1 は4ファイルを**独立抽選**するため、単に概念を細かく分割すればよいわけではありません。独立性が高い条件だけを別軸にし、組み合わせ依存が強い条件は1つのSceneとして扱います。

- `01_world.txt`: 「どんな舞台か」を単体でイメージできる候補にする。例: `湖畔`、`未来都市`、`雨の港町`、`紅葉の湖畔`
  - 天気や季節は `雨` / `秋` のように単独の独立軸へせず、Sceneを特徴づける場合だけ含める
  - 純粋な時間帯、配色、素材、光の演出、抽象的な気分や作業状態は入れない
- `02_time_of_day.txt`: `早朝`、`昼下がり`、`夕暮れ`、`深夜` のような時間帯だけにする
  - 季節、天候、月・星、休日/週末、静けさ、作業前後などを混ぜない
- `03_workspace_type.txt`: 図書館、ワークラウンジ、アトリエ、研究室など、実際の空間種別だけにする
  - 時間帯、天候、季節、色・素材・画風、座席配置を混ぜない
- `04_seat_layout.txt`: 並列、島型、リング、段差、ブース分散など、座席同士の位置関係を中心にする
  - UIカードの置きやすさ、画面上の安全地帯、レンダリング条件、時間・天候などを混ぜない

画風・レンダリング言語は **Direction / style**、配色・ライティング・材質感・空気感は **Look** の責務です。これらを Theme 候補へ混ぜません。抽象的な「静けさ」「ぬくもり」「爽やかさ」なども Theme の独立候補として増やさず、Scene・Direction・Look の組み合わせから立ち上げます。

現在の出力ラベル `世界観` は後方互換のため維持しますが、候補データ上の意味は上記の **Scene / World** として扱います。

### 共通用途制約・Direction・Look

| ファイル | 役割 |
|----------|------|
| `data/prompt_template.txt` | アスペクト比、UIを重ねる領域、人物・文字の禁止など、スタイルに依存しない共通用途制約。スタイル挿入位置として `{{STYLE}}` を1個だけ持つ |
| `data/style_legacy.txt` | 後方互換用の従来画風。手動管理 |
| `data/style_direction_*.generated.txt` | Direction Markdown の `## Prompt guidance` から生成されるCLI用style。**直接編集禁止** |
| `data/look_*.txt` | bundled Look profile。配色・ライティング・材質感・空気感を担当する。手動管理 |

共通テンプレート、Direction/style、Look、Theme は別々の責務です。最終的には Direction/style と Look を合成し、1つの visual guidance として `{{STYLE}}` に注入します。

### Direction の単一の正と生成

Direction A/B/C/D の canonical source は `.agents/skills/room-art-direction/references/direction-*.md` です。各Markdownの **`## Prompt guidance` セクションだけ**をCLI向けの実行用fragmentとして抽出します。Summary / Core / Avoid / Non-goals / Review checklist まで丸ごとCLIへ入れないため、Agent向け文書の表現力と実行プロンプトの簡潔さを両立します。

Direction D の最終品質確認は ChatGPT Chat / GPT-5.6 Sol / High を主基準とします。このCLIは最終画像を生成するものではなく、再利用可能なpromptを組み立てる役割です。Codex / Work / 軽量モデル固有の生成癖を補正するための文言は、canonical styleへ安易に追加しません。


### Look layer

Look は「何を描くか」でも「どうレンダリングするか」でもなく、**どの色・光・材質感・空気感で見せるか**を担当します。Direction A/B/C/D のいずれとも組み合わせられ、選択中の Direction の描画文法を上書きしません。

bundled Look:

| Look | 主な意図 |
| --- | --- |
| `indigo-violet-fantasy` | 青〜紫〜シアンを中心とした幻想的な色・光。暗部も色を保ち、発光感を出す |
| `airy-garden` | 空色・緑・アイボリー/ベージュ・温かい木色を軸にした爽やかな庭園系の配色 |
| `coral-aqua-glow` | コーラル/ピーチ/アンバーと澄んだアクア/ターコイズの暖冷対比。水がある場合は透明感を強調 |
| `crystal-lucent` | 淡いシアン・ラベンダー・乳白色を軸に、ガラス/クリスタル/半透明の軽やかな材質感を強調 |

Look は必ず、Theme で選ばれた時間帯・天候を尊重します。例えば `indigo-violet-fantasy` が昼を夜へ変えたり、`coral-aqua-glow` が朝を夕焼けへ変えたりしないよう、時間条件に応じて色・光の出し方だけを適応させます。

`auto` は上記4 Lookから一様に1つ選びます。Look選択は Theme と座席数の抽選**後**に同じ RNG から行うため、Look導入前と同じ `-seed` を指定しても既存4テーマと座席数の抽選順は変わりません。`-look none` を使えばLookを完全に外せます。

生成は次で行います。

```bash
cd tools/room-image-prompt
go generate ./data
```

生成済みファイルがcanonical Markdownと一致しない場合はGo testが失敗します。また、Direction referenceの変更でもRoom Image Prompt CIが起動するようpath routingで保護します。

出力に付与するテーマブロックの行ラベルは、順に `世界観` / `時間帯` / `作業空間` / `座席レイアウト` / `座席数` です（`internal/theme` の `FormatThemeBlock`）。`座席数` は10〜15の乱数で、専用の候補ファイルはありません。

### v1 の挙動と将来拡張

- **v1（現状）**: 4ファイル分は、当該ファイル内の候補から **一様な独立乱数** で1行ずつ選びます。`座席数` は 10〜15 から1回一様乱数で決めます。その後、Look 未指定時は bundled Look 4種から1つを一様に選びます。Theme と Look の相性スコアや条件付き再抽選は行いません。
- **将来拡張（未実装）の例**: 前段に応じた候補の重み付け、相性スコア、条件付き再抽選など。必要になったらアルゴリズムを差し替え可能な位置に集約する想定です。

### オプション

| フラグ | 説明 |
|--------|------|
| `-version` | バージョン表示して終了 |
| `-out <path>` | 出力ファイル（省略時は上記タイムスタンプ名） |
| `-seed <uint64>` | 乱数シード（10進）。省略時は非固定 |
| `-style <name>` | `legacy` または生成済み `direction-a` / `direction-b` / `direction-c` / `direction-d`。省略時は `legacy` |
| `-style-file <path>` | 任意の UTF-8 スタイル本文をファイルから読み込む。 `-style` と同時指定不可 |
| `-look <name>` | `auto` / `none` / bundled Look名。省略時は `auto` |
| `-look-file <path>` | 任意の UTF-8 Look guidance を読み込む。`-look` と同時指定不可 |

開発中は `go run` でも可です。

```bash
cd tools/room-image-prompt
go run ./cmd/room-image-prompt -version
go run ./cmd/room-image-prompt -seed 1
go run ./cmd/room-image-prompt -seed 1 -style legacy
go run ./cmd/room-image-prompt -seed 1 -style direction-a
go run ./cmd/room-image-prompt -seed 1 -style direction-b
go run ./cmd/room-image-prompt -seed 1 -style direction-c
go run ./cmd/room-image-prompt -seed 1 -style direction-d
go run ./cmd/room-image-prompt -seed 1 -style direction-d -look indigo-violet-fantasy
go run ./cmd/room-image-prompt -seed 1 -style direction-b -look airy-garden
go run ./cmd/room-image-prompt -seed 1 -style direction-d -look coral-aqua-glow
go run ./cmd/room-image-prompt -seed 1 -style direction-c -look crystal-lucent
go run ./cmd/room-image-prompt -seed 1 -style direction-a -look none
go run ./cmd/room-image-prompt -seed 1 -style-file ./my-style.txt -look-file ./my-look.txt
```

## テスト

```bash
cd tools/room-image-prompt
go test ./...
```

## ライセンス

リポジトリ全体のライセンスに従います。
