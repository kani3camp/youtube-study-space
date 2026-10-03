# Firestore scheduled export

2026-10-03 に deployed Cloud Functions version 5 の source を復元。development / production は project ID と bucket URI の literal 以外同じで、package.json は同一だった。元 archive は index.js / package.json のみ。lockfile は存在しなかったため、当時の transitive dependency の exact version は再現できない。

現行契約: (default) database の users / user-activities / order-history のみ export。payload は無視し、LRO を開始するだけで完了を待たない。API reject は console.error 後に undefined へ正常 resolve する。

| 環境 | Function | region | version 5 archive SHA-256 |
| --- | --- | --- | --- |
| development | firestoreCollectionsExport | asia-southeast2 | 19abaa897b3c1b017aa6b033c0404814c6433cc0bfa40029f76222efbd76b33c |
| production | firestoreExport | asia-northeast2 | 7d0aaf2356da3e4e80ce426f8040b17726d74c7bcbc51b46ca2d2ca8c41c467e |

この commit の index.js / package.json は development の復元 baseline そのもの。以後の commit で単一 source の環境設定と Node.js 22 準備を追加する。cloud 変更は行わない。
