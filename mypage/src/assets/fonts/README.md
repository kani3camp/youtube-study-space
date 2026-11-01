# M PLUS Rounded 1c

承認済みプロトタイプが指定する400/500/700/800の同じフォントをローカル配信する。CSSのファミリー名だけでは読み込まれないため、`fonts.css`をメインのスタイルシートからimportする。実行時にGoogle Fontsへ外部リクエストする必要はない。

公式Google Fonts CSS2 APIが配信するv22のTTFをWOFF2へ変換し、後述の描画テーブルを補正した。サブセット化はしていない。全8,532グリフの順序・cmap・横方向のメトリクス・nameレコードを往復比較し、取得元URLと入出力のSHA-256は[sources.json](sources.json)に保持する。内部PostScript名はRoundedMplus1c-Regular/Medium/Bold/ExtraBoldであり、CSS上のファミリー名とは異なる。

4ファイルのサイズとSHA-256はsources.jsonを参照する。ブラウザは使用したウェイトだけを取得する。Hosting候補は現在すべてのアセットに`Cache-Control: no-store`を設定しており、HTTPキャッシュで再利用しない。静的フォントのキャッシュ対応は公開前の改善候補として残し、認証・非公開レスポンスのキャッシュポリシーは変更しない。`font-display: swap`で初回読み込み中やアセット取得失敗時も本文を表示する。描画比較やQAは`document.fonts.ready`の後に行い、実際のカスタムフォントとPostScript名をChromium CDPで検証する。

Copyright 2016 The Rounded M+ Project Authors. フォントだけにSIL Open Font License 1.1を適用し、[著作権・ライセンス全文](../../../public/fonts/mplus-rounded-1c/OFL.txt)をビルドにも同梱する。[公式ライセンス](https://raw.githubusercontent.com/google/fonts/main/ofl/roundedmplus1c/OFL.txt)はソフトウェアへの同梱・再配布を許可し、著作権表示とライセンスの保持を要求する。フォント単体で販売しない。著作権表示にReserved Font Nameの指定はない。

ブラウザ検証には`python3 scripts/font-qa.py --output /tmp/mypage-font-qa`を使う。ローカルVite（18081）とChromium/Playwrightが必要であり、通常読み込みとフォント取得失敗時の代替表示を確認する。

## 小さい文字の平滑化

2026-10-21に描画設定を補正した。元の4ウェイトは`gasp` version 0 / `{65535: 0x0002}`だった。接続ブラウザで同じ字形・幅・サイズを比較し、ユーザーが滑らかと確認した2列目のversion 1 / `{65535: 0x000A}`を全ウェイトに適用する。グレースケールと対称なClearType平滑化を指定し、hintingは追加しない。CSSの文字サイズやウェイトは変更しない。全OSの描画結果を保証する設定ではない。

これは公式フォントの新バージョンではなく、配信アセットへのローカルな描画テーブル補正である。`gasp`と`head.checkSumAdjustment`以外の全テーブルを元のWOFF2と比較し、字形・cmap・文字幅・name・ライセンス・時刻が保持されることを確認した。元WOFF2のSHA-256、保持テーブルを連結したSHA-256、補正後のSHA-256をsources.jsonに記録する。

再現には補正前のWOFF2（統合コミット`733c34e50f4bab48d3b95c3efff71e17d7aa954e`）と`font-requirements.txt`の環境を使う。入力を`originalWOFF2SHA256`で照合してから、FontToolsの`TTFont(path, recalcTimestamp=False)`で開き、`font['gasp'].version = 1`と`font['gasp'].gaspRange = {65535: 0x000A}`だけを設定して保存する。

```sh
python3 -m pip install -r font-requirements.txt
python3 scripts/check-fonts.py
```

このオフライン検査はフロントエンドCIでも実行する。4ウェイトの平滑化設定・バイト列とハッシュ・元テーブルの保持を検証し、古い描画設定への戻りや字形・幅の変化を検出する。保持テーブルのダイジェストはtag順に`tag(ASCII) + length(4-byte big-endian) + table bytes`を連結する。`gasp`は除外し、`head`のoffset 8..11のchecksumだけを0に正規化する。
