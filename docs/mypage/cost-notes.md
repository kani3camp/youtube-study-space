# MyPage増分原価の前提

[入力値](cost-model.json)と[再生成可能な試算](cost-scenarios.md)を正とする。`python mypage/scripts/cost-model.py --test`で会計・構成の回帰を検査し、`--input`で感度を変え、`--output docs/mypage/cost-scenarios.md`で表を生成する。請求書・公開価格・公開利用枠の確定ではない。

## 比較する構成

`hosting-run`は現行Firebase Hosting（静的配信とAPI rewrite）＋API Cloud Run。`armor-run`はNotion 06の暫定候補であるGlobal LB＋同一Armor policy・8 rules＋Web/APIの2 Cloud Run、CDNなし。後者だけに本番固定$31.25と常設DEV固定$31.25を計上する。構築済みとは扱わない。静的ファイルにもRun request/割当CPU・RAM、WAF、LB入出力、Internet送信を計上し、Hosting転送と重ねない。browser cacheはCDNと別で、HTMLは毎回no-storeのまま。

標準は200 MyPage DAU・visible 2時間/日・30日。初期取得を含み60秒726,000→5分150,000 requests/月。route外・hidden・offlineの削減、singleflight/cache hitは未測定なので追加控除しない。Bot DAUは別入力で、月次writer追加費をMyPage利用者だけに限定しない。小規模ではwriterの追加費がraw削減を上回る場合があり、月次方式を無条件に安いとしない。

## 確認した値と未測定の値

- 2026-10-24 JSTの公式地域選択でFirestore Standard read $0.038/10万、write $0.115/10万、delete $0.013/10万を大阪・ジャカルタで確認。保存は大阪$0.195/GiB月、ジャカルタ$0.192。旧Iowa $0.03を流用しない。個別SKU ID・実請求との照合は未完。
- Runは大阪CPU $0.000024/vCPU秒・RAM $0.0000025/GiB秒、ジャカルタは各1.4倍。request $0.40/100万。0.2秒は**割当資源の課金wall time/requestの仮定**で、process CPUの測定値ではない。100ms丸め・起動・並行処理を実測する必要がある。Web配信0.05秒/requestも仮定。
- 月次readerはEmulatorでraw query 0・最大2月lookupを確認し、[DEV実測](dev-cost-measurement-20261024.md)でもraw 0/10/100/500/1000/1001とJST境界でraw query 0・この日付では1月lookupを確認した。平均1.2は未測定。認可/metadataの6 SAC＋WebAccount1、snapshotのUser1＋2 seat query最低課金を加え、cache missは通常11～12 read相当。cache hitにも4 SACを計上。transaction retry、metadata更新、Auth、App Check等は別で、請求上限の保証ではない。
- 実DEVの同日fixtureではraw返却0/10/100/500/1000/1001に対しAnalyzeのdocument scanも同数、index scanも同数だった。0件queryのreadOperationsは1、1,000件では1,001、1,001件では1,003だった。実利用者の分布は測っていないため、raw平均20文書/miss、index 1,000 entries/missという既定シナリオは比較用仮定として残す。空query最低1 readとindex 1,000件単位の切上げを別計算する。limit=1001をindex scan課金上限とみなさない。Query Explain Analyzeも通常query料金が発生する。
- 月次writerはcloseあたり追加read 5.05・write 2.05を仮置き（再実行・月跨ぎ比率を含む推定）。raw自体の既存書込費は既存利益側に含め、増分に重ねない。全Bot DAUを対象にする。競合retry、移行・rebuild・削除・障害復旧は通常closeの外で別予算。
- 明細は既定50/max100件、候補index 20,000 entries、control read推定32、quota reserve/release write 2。Firestore推定はpage size＋sentinel＋controlとindexを別算出。予約103 weighted units（最大pageなら153）は**実課金read数ではない**。複数attempt・失敗・ACK不明の増分は実測が必要。応答32KiBは仮定で、最大側の感度は128KiB。明細費表はRun/WAF/通信/管理費を含む総額差分で、Firestore専用値と区別する。分析・出力機能の実装や無制限Premiumを意味しない。
- DEV release 1792827165956000では、JS/CSS/4 fontの正確なhash付きpathに1年immutableを配備し、8 filesのbody SHA256一致とHTML/API/未知hashのno-storeを確認した。4 fontのsource合計4,278,928 bytesはwire実測ではない。1 documentあたり4.5MB/7 requests、cache後平均0.2MB/1.2 requestsは未測定の仮定。初回訪問・更新releaseでは再downloadがあり、0.2/1.5/4.5MB感度を併記する。
- App Checkは1時間TTLの約半分でrefreshするSDK挙動から2 assessments/滞在時間を仮置きし、pollingと独立計算。既存organization利用10,000を加え、reCAPTCHAのorganization共有課金の**増分**を求める。成長シナリオではこの費用が支配的であり、5分pollだけで1/5としない。

## 無料枠・未計上・採算

既存利益は利用者申告の新Web前月30,000円を1回だけ使用。候補690円の新規加入は690×手取り率、290円からのupgradeは差額400×手取り率。手取り率0.7・為替150円/USD・税倍率1（税別）は仮定で、実際の手数料・税・返金・事業契約を確認する。維持人数は新Web増分全額を埋め、既存30,000円を維持する人数。

Firestore無料枠はprojectの対象DB・日単位でBot等と共有。Run無料枠は**請求先アカウント全projectで共有**（Tier1換算CPU $4.32、RAM $0.90、requests $0.80/月）。本モデルの利用可能残は既定0で、Production側だけから控除しDEVにも二重適用しない。DEV変動費は独立入力で既定0、今回の合成試験は承認された一時予算で別管理する。

Firebase HostingはGB、LB/FirestoreはGiBで換算。Hosting公式料金表の360MB/日とusage文書の10GB/月に無料転送枠表記の相違があるため、既定0とする。全site/channelと旧release保存量を合算して確認する。Loggingは既存費を差し引いた新規増分だけを計算する。

新規Firestore storage、Hosting release storage、通常追加ログは未測定で既定0。値が0だから無料という意味ではない。index/文書サイズ、旧release保持、30日超ログ保持、backup/PITR、migration/deletion、Auth/Identity Platformの実課金条件、Monitoring custom metrics、YouTube quota、abuse traffic、DEV変動費、税を別途確認する。モデルは記載範囲の増分予算であり、実構成の完全な請求総額ではない。通常既存workloadは既存利益に含む。

DEVのON/OFF・409競合拒否・合成削除・一時権限解除は[実測記録](dev-cost-measurement-20261024.md)を参照。1,000件fixtureのraw query部分を月次1文書へ置換しても、API全体が1 readになるわけではない。owner端末→JakartaのSDK経過時間はCloud Run CPU・課金wall timeではない。App Checkの1時間TTLを確認し、同じ日単位key集計の前後66→66を取得したが、反映遅延と集計粒度のため評価回数0やpolling別単価の実証とはしない。

公式出典は[inputのsources](cost-model.json)へ集約。実測・配備・残Gateは既存[Issue #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)で追跡する。
