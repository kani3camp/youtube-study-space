# M PLUS Rounded 1c（ローカルフォント配信）

承認済みプロトタイプが指定する 400 / 500 / 700 / 800 のウェイトをローカルから配信するためのフォント資産です（CSS のファミリー名だけでは読み込まれないため、`fonts.css` をメインスタイルシートから import し、実行時の Google Fonts への外部リクエストを不要にしています）。

## 収録内容とライセンス

- 変換元と整合性: 公式 Google Fonts CSS2 API が配信する v22 の TTF を WOFF2 へ変換し、後述の描画テーブル補正を施しています（サブセット化はしていません）。全 8,532 グリフの順序・`cmap`・横メトリクス・`name` レコードを比較検証済みで、取得元 URL・サイズ・入出力の SHA-256 は [sources.json](sources.json) に記録しています（内部 PostScript 名は `RoundedMplus1c-Regular` / `Medium` / `Bold` / `ExtraBold`）。
- 配信と表示: ブラウザは使用したウェイトのみを取得し、`font-display: swap` により読み込み中や取得失敗時も本文を表示します。描画比較や QA は `document.fonts.ready` の完了後に行い、Chromium CDP で実際の適用フォントと PostScript 名を確認します（※現在 Hosting 設定候補はアセットに `Cache-Control: no-store` を付けており、静的フォントのキャッシュ化は公開前の改善候補です）。
- ライセンス: Copyright 2016 The Rounded M+ Project Authors。フォント単体に SIL Open Font License 1.1 が適用され、ビルドにも [OFL.txt](../../../public/fonts/mplus-rounded-1c/OFL.txt)（[公式ライセンス](https://raw.githubusercontent.com/google/fonts/main/ofl/roundedmplus1c/OFL.txt)）を同梱しています。フォント単体での販売は禁止されており、著作権表示とライセンス保持が必要です（Reserved Font Name の指定はありません）。

## ブラウザ確認と小さい文字の平滑化（`gasp` 補正）

2026-10-21 に小さい文字の可読性を改善するため描画設定を補正しました（公式フォントの新バージョンではなく、ローカルな描画テーブル補正です）。

- 元の 4 ウェイトは `gasp` version 0 / `{65535: 0x0002}` でしたが、接続ブラウザで同じ字形・幅・サイズを比較したうえで滑らかに表示された 2 列目の `version 1 / {65535: 0x000A}`（グレースケール＋対称 ClearType 平滑化、hinting 追加なし）を全ウェイトに適用しました（※全 OS で同一の描画結果を保証するものではありません）。
- `gasp` と `head.checkSumAdjustment` 以外の全テーブルが元 WOFF2 と一致することを検証しています（保持テーブルのダイジェストは tag 順に `tag(ASCII) + length(4-byte big-endian) + table bytes` を連結し、`gasp` を除外して `head` の offset 8..11 の checksum だけを 0 に正規化して算出）。再現する場合は補正前の WOFF2（統合コミット `733c34e50f4bab48d3b95c3efff71e17d7aa954e`）と `font-requirements.txt` を用意し、`originalWOFF2SHA256` 照合のうえ FontTools の `TTFont(path, recalcTimestamp=False)` で `gasp` を更新します。

```sh
# 整合性チェック（フロントエンド CI でも実行）
python3 -m pip install -r font-requirements.txt
python3 scripts/check-fonts.py

# ブラウザ描画検証（ローカル Vite 18081 + Chromium / Playwright 環境が必要）
python3 scripts/font-qa.py --output /tmp/mypage-font-qa
```
