# MyPage 増分原価モデル

[入力値](cost-model.json)と [calculator](../../mypage/scripts/cost-model.py) で、利用量・構成・集計方式を変えた試算を生成する。入力の `verifiedAtJST` は価格確認日であり、現在の料金や請求実績を保証しない。再利用時は対象 region と公式料金を再確認する。公式出典は入力の `sources` に集約する。

```sh
python3 mypage/scripts/cost-model.py --test
python3 mypage/scripts/cost-model.py --output /tmp/mypage-cost-report.md
# 感度を変える場合は入力の private copy を作り --input で指定する。
```

生成表は作業資料として扱い、repository へ重複保存しない。請求照合と未完の実測は [公開受入 #1133](https://github.com/kani3camp/youtube-study-space/issues/1133) で追跡する。

## 構成と入力の意味

- 採用構成は `hosting-run`（Firebase Hosting + Cloud Run API）。`armor-run` は不採用の LB / Armor / Run Web 比較シナリオで、calculator の先頭表はこの旧構成の感度比較。採用構成の値は `hosting-run` の行を使う。構築状態・承認済み予算を示す表ではない。
- DAU、滞在時間、raw 件数・index entry、月 lookup 平均、cache hit、課金 wall time、転送量、writer retry はすべて試算入力。小規模な合成 fixture や端末からの SDK latency を利用者平均・Run CPU・請求値へ一般化しない。
- overview、session 完了 POST、明細、全 Bot 利用者の月次 writer を別々に加算する。5分 polling への変更が session 回数や App Check assessment を同じ割合で減らすとは仮定しない。
- 月次 lookup は最大2文書だが、API 全体が2 readになる意味ではない。認可・WebAccount・User・空 seat query の最低課金、metadata 更新、transaction retry、index entry を含めて評価する。cache hit でも認可 read は必要。
- `limit(1001)` は index scan の課金上限ではない。Query Explain Analyze 自体も課金対象。明細の weighted quota は実 read 数と区別し、失敗・ACK 不明・quota 管理の増分も計上する。
- Run の秒数は割当資源の課金 wall time の仮定。100ms 丸め・起動・同時実行を別に確認する。static cache は初回訪問・release 更新時の再 download を残し、HTML/API の no-store と分ける。
- App Check / reCAPTCHA は organization 全体の assessment に対する増分。polling と独立し、別 project の利用も確認する。

## 会計と除外

Firestore 無料枠は project の対象 DB・日単位、Run 無料枠は請求先 account の全 project で共有。Hosting は全 site/channel と旧 release 保存量を合算する。利用可能残は入力で明示し、DEV/PROD に二重控除しない。Hosting の GB と Firestore / LB の GiB を区別する。

既存 workload は baseline 利益に含め、新 Web 増分だけを引く。候補会費・手取り率・為替・税は仮定で、商品価格の承認ではない。upgrade は既存会費との差額だけを収益に加える。Bot writer と DEV の変動費・image 保存も別に加算する。

storage、index、古い release、ログ保持、backup/PITR、migration/deletion、Auth / Identity Platform、Monitoring、YouTube quota、abuse、税・手数料・返金は必要に応じて追加する。入力が0でも無料の証明にはならず、このモデルを完全な請求総額や費用 hard cap と呼ばない。

実運用の instance 上限・Budget 実着信・異常利用監視は [release gate](release/README.md) と [E2E](release/e2e.md) で確認する。
