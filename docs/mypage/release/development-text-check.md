# DEV small text readability

## Task contract

Goal: MyPageの小さい説明文・日付を読みやすくする。
Invariants: ApprovedのM PLUS Rounded 1c、見出し・主要数値・色・操作・Auth/CSPを維持する。実字形の変更、フォント追加、OS設定変更を行わない。
Non-goals: Production変更、履歴coverageの変更、dev/mainへのmerge。
Acceptance: 説明文14px/500、補助説明13px/500、日付12px/500を基本とし、mobileで説明文を11pxへ戻さない。320/390pxとPCで折り返し・横はみ出し・アカウント操作を確認する。
Verification: 既存frontend lint/type/test/buildとCI、DEV配信後のcomputed styles・font load・表示を確認。Windowsの描画改善を定量的に保証したとは記録しない。

## Findings (2026-10-21 JST)

添付画面では細い補助文字の輪郭が目立つ。DEVで400/500/700/800のfont-face登録とload済み状態を確認し、対象文字のtransformはnone。小さい説明は12px/400、日付は11px/400、mobile集計の説明は11pxに縮小されていた。接続ブラウザのdevicePixelRatioは1.25、visual viewport scaleは1であり、OS表示倍率とブラウザzoomの内訳までは断定できない。

フォント未取得・CSS変形が原因という証拠はない。元フォントのgaspもグレースケールを許可しており、font binaryを加工しない。細い字形と小さいサイズを改善対象とし、使用済みのMedium (500)を明示する。補助説明の行数・カード高さが増えることを許容し、情報の省略・圧縮で読みやすさを戻さない。

見出し・主要数値のRounded 1cの印象は維持する。CSS font smoothingはWindows向けの確実な改善策ではないため追加しない。実描画は端末・表示倍率・ブラウザに依存し、変更後もジャギー解消そのものはユーザー環境での確認を要する。

配信と実画面の確認結果は本変更PRの最新本文へ記録する。
