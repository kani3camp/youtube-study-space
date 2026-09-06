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

引数なしで、上から4テーマ行（各 `data/0N_*.txt` から1行を独立に乱数抽選）に加え、**座席数 10〜15** を1回一様乱数で決定します。`data/prompt_template.txt` の `{{STYLE}}` に既定の `data/style_legacy.txt` を挿入し、最後にテーマ条件を連結した UTF-8 テキストを **`output/prompt-<タイムスタンプ>.txt`** に書き込みます。あわせて**同じ本文をクリップボードへコピー**します（失敗しても終了コードは成功のままです）。

`-style direction-a` / `direction-b` / `direction-c` / `direction-d` で管理人のアートディレクションを選択できます。`-style-file <path>` を指定すると、任意の UTF-8 テキストをスタイルとして注入できます。未指定時は後方互換のため `legacy` です。

- **標準エラー**: `出力: <ファイル名>` に続き、`クリップボードにコピーしました` または `コピーに失敗しました` を1行ずつ出します。Linux などで `xclip` / `xsel` が無い環境ではコピーが失敗し得ます。
- **標準出力**: **保存したファイルの絶対パスを1行**だけ出します（スクリプト向け）。

実行時は **`tools/room-image-prompt` をカレントディレクトリにした状態**で動かす想定です（省略時の出力先が `./output/` になるため）。

### テーマ候補ファイル（ステップ順）

| ファイル | 内容の例 |
|----------|-----------|
| `data/01_world.txt` | 世界観（天気・季節・質感・雰囲気など） |
| `data/02_time_of_day.txt` | 時間帯 |
| `data/03_workspace_type.txt` | 実際の作業空間の種類 |
| `data/04_seat_layout.txt` | 座席レイアウト |

座席数（10〜15）は専用ファイルを使わず、毎回一様乱数で1つ選びます。

### 共通用途制約とスタイル

| ファイル | 役割 |
|----------|------|
| `data/prompt_template.txt` | アスペクト比、UIを重ねる領域、人物・文字の禁止など、スタイルに依存しない共通用途制約。スタイル挿入位置として `{{STYLE}}` を1個だけ持つ |
| `data/style_legacy.txt` | 後方互換用の従来画風。手動管理 |
| `data/style_direction_*.generated.txt` | Direction Markdown の `## Prompt guidance` から生成されるCLI用style。**直接編集禁止** |

共通テンプレートの読み込み、style sourceの選択、style適用は別々の責務です。

### Direction の単一の正と生成

Direction A/B/C/D の canonical source は `.agents/skills/room-art-direction/references/direction-*.md` です。各Markdownの **`## Prompt guidance` セクションだけ**をCLI向けの実行用fragmentとして抽出します。Summary / Core / Avoid / Non-goals / Review checklist まで丸ごとCLIへ入れないため、Agent向け文書の表現力と実行プロンプトの簡潔さを両立します。

Direction D の最終品質確認は ChatGPT Chat / GPT-5.6 Sol / High を主基準とします。このCLIは最終画像を生成するものではなく、再利用可能なpromptを組み立てる役割です。Codex / Work / 軽量モデル固有の生成癖を補正するための文言は、canonical styleへ安易に追加しません。

生成は次で行います。

```bash
cd tools/room-image-prompt
go generate ./data
```

生成済みファイルがcanonical Markdownと一致しない場合はGo testが失敗します。また、Direction referenceの変更でもRoom Image Prompt CIが起動するようpath routingで保護します。

出力に付与するテーマブロックの行ラベルは、順に `世界観` / `時間帯` / `作業空間` / `座席レイアウト` / `座席数` です（`internal/theme` の `FormatThemeBlock`）。`座席数` は10〜15の乱数で、専用の候補ファイルはありません。

### v1 の挙動と将来拡張

- **v1（現状）**: 4ファイル分は、当該ファイル内の候補から **一様な独立乱数** で1行ずつ選びます。`座席数` は 10〜15 から1回一様乱数で決めます。前段の抽選に依存する重み付けや、相性スコアによる再抽選は行いません。
- **将来拡張（未実装）の例**: 前段に応じた候補の重み付け、相性スコア、条件付き再抽選など。必要になったらアルゴリズムを差し替え可能な位置に集約する想定です。

### オプション

| フラグ | 説明 |
|--------|------|
| `-version` | バージョン表示して終了 |
| `-out <path>` | 出力ファイル（省略時は上記タイムスタンプ名） |
| `-seed <uint64>` | 乱数シード（10進）。省略時は非固定 |
| `-style <name>` | `legacy` または生成済み `direction-a` / `direction-b` / `direction-c` / `direction-d`。省略時は `legacy` |
| `-style-file <path>` | 任意の UTF-8 スタイル本文をファイルから読み込む。 `-style` と同時指定不可 |

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
go run ./cmd/room-image-prompt -seed 1 -style-file ./my-style.txt
```

## テスト

```bash
cd tools/room-image-prompt
go test ./...
```

## ライセンス

リポジトリ全体のライセンスに従います。
