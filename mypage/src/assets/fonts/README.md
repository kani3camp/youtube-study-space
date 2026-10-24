# M PLUS Rounded 1c

Approved prototypeが指定する400/500/700/800の同一fontをlocal配信する。CSSのfamily名だけではfontは読み込まれないため、`fonts.css`をmain stylesheetからimportする。runtimeのGoogle Fonts外部requestは不要。

公式Google Fonts CSS2 APIが配信するv22のTTFをWOFF2へformat変換し、後述の描画tableを補正した。subsettingはしていない。全8,532 glyphのorder/cmap/horizontal metrics/name recordsを往復比較し、source URLと入力/出力SHA-256は[sources.json](sources.json)に保持する。内部PostScript名はRoundedMplus1c-Regular/Medium/Bold/ExtraBoldで、CSS上のfamily名と異なる。

4ファイルのサイズとSHA-256はsources.jsonを参照。browserは使ったweightだけを取得する。Hosting候補は現在すべてのassetに`Cache-Control: no-store`を設定しており、HTTP cacheによる再利用は行わない。静的fontのcache対象化は公開前の改善候補として残し、認証/private responseのcache policyは変更しない。`font-display: swap`で初回load中やasset失敗時にも本文を表示する。描画比較やQAは`document.fonts.ready`後に実際のcustom fontとPostScript名をChromium CDPで検証する。

Copyright 2016 The Rounded M+ Project Authors. fontのみSIL Open Font License 1.1を適用し、[copyright/license全文](../../../public/fonts/mplus-rounded-1c/OFL.txt)をbuildにも同梱する。[公式license](https://raw.githubusercontent.com/google/fonts/main/ofl/roundedmplus1c/OFL.txt)はsoftwareへのbundling/redistributionを許可し、noticeとlicenseの保持を要求する。font単体での販売はしない。copyright statementにReserved Font Name指定はない。

browser検証: `python3 scripts/font-qa.py --output /tmp/mypage-font-qa`。local Vite（18081）とChromium/Playwrightが必要。通常loadとfont request失敗時のfallbackを確認する。

## 小さい文字の平滑化（2026-10-21）

元の4 weightは`gasp` version 0 / `{65535: 0x0002}`だった。接続ブラウザで同じ字形・幅・サイズの比較を行い、ユーザーが滑らかと確認した2列目のversion 1 / `{65535: 0x000A}`を全weightへ適用する。グレースケールと対称なClearType平滑化を指定し、hintingは追加しない。CSSの文字サイズ・weightは変更しない。全OSの描画結果を保証する設定ではない。

これは公式fontの新バージョンではなく、配信assetへのローカルな描画table補正。`gasp`と`head.checkSumAdjustment`以外の全tableを元WOFF2と比較し、字形・cmap・文字幅・name・license・timestampが保持されることを確認した。元WOFF2のSHA-256、保持tableの連結SHA-256、補正後のSHA-256をsources.jsonに記録する。

再現には補正前のWOFF2（integration commit `733c34e50f4bab48d3b95c3efff71e17d7aa954e`）と`font-requirements.txt`の環境を使う。入力を`originalWOFF2SHA256`で照合したうえで、FontToolsの`TTFont(path, recalcTimestamp=False)`を開き、`font['gasp'].version = 1`と`font['gasp'].gaspRange = {65535: 0x000A}`だけを設定して保存する。

```sh
python3 -m pip install -r font-requirements.txt
python3 scripts/check-fonts.py
```

このoffline checkをfrontend CIでも実行する。4 weightの平滑化設定、byte/hash、元tableの保持を検証し、古い描画設定への戻りや字形・幅の変化を検出する。保持tableのdigestはtag順に`tag(ASCII) + length(4-byte big-endian) + table bytes`を連結し、`gasp`を除外、`head`のoffset 8..11のchecksumだけを0に正規化する。
