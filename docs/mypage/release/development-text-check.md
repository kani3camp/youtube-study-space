# DEV small text readability

## Task contract

Goal: MyPageの小さい説明文・日付を読みやすくする。
Invariants: ApprovedのM PLUS Rounded 1c、見出し・主要数値・色・操作・Auth/CSPを維持する。実字形の変更、フォント追加、OS設定変更を行わない。
Non-goals: Production変更、履歴coverageの変更、dev/mainへのmerge。
Acceptance: 説明文14px/500、補助説明13px/500、日付12px/500を基本とし、mobileで説明文を11pxへ戻さない。320/390pxとPCで折り返し・横はみ出し・アカウント操作を確認する。
Verification: 既存frontend lint/type/test/buildとCI、DEV配信後のcomputed styles・font load・表示を確認。Windowsの描画改善を定量的に保証したとは記録しない。

## Findings (2026-10-21 JST)

添付画面では細い補助文字の輪郭が目立つ。DEVで400/500/700/800のfont-face登録とload済み状態を確認し、対象文字のtransformはnone。小さい説明は12px/400、日付は11px/400、mobile集計の説明は11pxに縮小されていた。接続ブラウザのdevicePixelRatioは1.25、visual viewport scaleは1であり、OS表示倍率とブラウザzoomの内訳までは断定できない。

フォント未取得・CSS変形が原因という証拠はない。初回は元フォントのgaspがグレースケールを許可していることから、font binaryを加工しない方針を取った。後述の実機比較でこの判断を更新した。細い字形と小さいサイズを改善対象とし、使用済みのMedium (500)を明示する。補助説明の行数・カード高さが増えることを許容し、情報の省略・圧縮で読みやすさを戻さない。

見出し・主要数値のRounded 1cの印象は維持する。CSS font smoothingはWindows向けの確実な改善策ではないため追加しない。実描画は端末・表示倍率・ブラウザに依存し、変更後もジャギー解消そのものはユーザー環境での確認を要する。

配信と実画面の確認結果は本変更PRの最新本文へ記録する。

## Narrow date labels

初回DEV実確認で390pxは横はみ出しなし、320pxでは12pxの日付幅が7列のセル幅を約3px上回って隣の日付と重なった。日付spanをセル幅以内に制限し、狭い場合は月日の間などで折り返す。12px/500を維持し、文字の縮小・省略や列の重なりで収めない。PCの幅に余裕がある場合は引き続き1行表示する。320/390/768/1024/1440pxで日付矩形の非交差と説明文の非clip・全体の横はみ出しを実確認する。

## Rendering comparison and selected fix

サイズ・weightの調整後もユーザー環境で輪郭のぎざつきが残った。元assetは全weightでgasp v0 / 0x0002。字形・幅・サイズを固定し、元asset、v1 / 0x000A、v1 / 0x000F、native fontの4列を比較した。ユーザーから「1列目以外はぎざつきを感じない」「2列目で十分」と確認を得たため、v1 / 0x000Aを400/500/700/800へ採用する。

Goal: 選択された2列目の平滑化をDEVのMyPageへ反映する。
Invariants: gaspとhead checksum以外の全table、字形・文字幅・CSS・操作・Auth/CSP・licenseを保持する。
Non-goals: hinting追加、OS設定変更、Production変更、dev/mainへのmerge。
Acceptance: 全4 weightがgasp v1 / FFFF:000A、元tableを保持し、DEV配信byteとloadを確認する。
Verification: 全tableのdigest、常設offline check、frontend lint/type/test/build、CI、実DEVのfont loadと320/390/1440px表示を確認する。実機比較の結果を根拠とし、全端末での描画保証とは扱わない。
