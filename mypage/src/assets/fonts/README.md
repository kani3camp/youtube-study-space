# M PLUS Rounded 1c

Approved prototypeが指定する400/500/700/800の同一fontをlocal配信する。CSSのfamily名だけではfontは読み込まれないため、`fonts.css`をmain stylesheetからimportする。runtimeのGoogle Fonts外部requestは不要。

公式Google Fonts CSS2 APIが配信するv22のTTFをWOFF2へformat変換した。subsettingはしていない。全8,532 glyphのorder/cmap/horizontal metrics/name recordsを往復比較し、source URLと入力/出力SHA-256は[sources.json](sources.json)に保持する。内部PostScript名はRoundedMplus1c-Regular/Medium/Bold/ExtraBoldで、CSS上のfamily名と異なる。

4ファイル合計4,278,428 bytes。browserは使ったweightだけを取得しcacheする。`font-display: swap`で初回load中やasset失敗時にも本文を表示する。描画比較やQAは`document.fonts.ready`後に実際のcustom fontとPostScript名をChromium CDPで検証する。

Copyright 2016 The Rounded M+ Project Authors. fontのみSIL Open Font License 1.1を適用し、[copyright/license全文](../../../public/fonts/mplus-rounded-1c/OFL.txt)をbuildにも同梱する。[公式license](https://raw.githubusercontent.com/google/fonts/main/ofl/roundedmplus1c/OFL.txt)はsoftwareへのbundling/redistributionを許可し、noticeとlicenseの保持を要求する。font単体での販売はしない。copyright statementにReserved Font Name指定はない。

browser検証: `python3 scripts/font-qa.py --output /tmp/mypage-font-qa`。local Vite（18081）とChromium/Playwrightが必要。通常loadとfont request失敗時のfallbackを確認する。
