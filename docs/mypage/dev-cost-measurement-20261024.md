# 2026-10-24 DEV原価測定・復旧記録

対象は `test-youtube-study-space / asia-southeast2`。実装は専用の `integration/mypage-canon-20261006` へ統合し、Productionとbase=`dev`のPR #1236は変更・mergeしていない。採算の入力は[cost-model.json](cost-model.json)、生成表は[cost-scenarios.md](cost-scenarios.md)、数値証跡は[DEV evidence JSON](dev-cost-evidence-20261024.json)を参照する。

## 結果と確認範囲

同日raw 0/10/100/500/1000/1001件と、秒未満を含むJST日・月跨ぎ2件を、既存SnapshotReaderとAggregateで実Firestoreへ接続して測定した。月次readerは全fixtureでraw query 0、この日付では1月文書を1回lookupした。0～1000件で記録済み秒数はrawと一致し、1001件ではrawの`HISTORY_LIMIT_EXCEEDED`と月次の記録済み1001秒を区別した。coverage不明をexactな今日/今週の0へ変換しない。境界fixtureの再構築は日別丸め後の月内合計0秒、readerは直近日跨ぎのcarryを含む週の記録済み1秒、累計2秒となった。ただし月跨ぎ原記録は10/1境界で、10/24時点の直近7日window外にある。月跨ぎ原記録の作成・月内再構築は確認できたが、月跨ぎを含む直近windowのraw/月次reader比較と実2月lookupは未実証で、Emulatorの受入とは区別する。

これはowner端末からJakartaへ接続するSDK測定で、HTTP BFF・Cloud Run CPU・課金wall time・cache命中率の計測ではない。raw返却文書数を課金read数と呼ばない。取得失敗・並行writer・Privacy競合・明細の複数instance quotaなどは既存CI/Emulatorの検証で、実DEV障害注入済みとはしていない。

Query Explainは先にAnalyzeなしでCollectionのUID先頭composite `(user-id ASC, ended-at ASC, started-at ASC, __name__ ASC)` を確認し、対象の合成原記録だけを所有revision/hashで照合してからAnalyzeした。空queryはreadOperations=1、1000件ではdocument scan=1000 / index scan=1000 / readOperations=1001、1001件では各1001 / 1001 / 1003だった。Analyze自体も課金対象。別の明細用index・policy・quotaの実DEV検証に流用しない。

raw queryの1000文書を月次1文書へ置換した部分だけなら99.9%減。認可等のsource inventoryである6 SAC + WebAccount1 + User1 + 空seat query最低課金2を加え、Explain値を同条件へ当てはめたcache miss推定は1011→11 read相当（約98.9%減）となる。これはAPIの実請求や利用者平均の測定値ではない。writer追加read/write、App Check、静的配信、Run等を含むTCOは別表で計算する。母集団分布が未測定なので既定のraw平均20・月lookup平均1.2・index1000は仮定のまま残し、fixture感度表では実測index scanへ差し替える。

## 実配備した対

| 項目 | 固定した値・証跡 |
| --- | --- |
| image build source | `5803ad18c418307b8fdc7de7f7ce3e0ddc0fe774`、[既存CI image build](https://github.com/kani3camp/youtube-study-space/actions/runs/40064524012) |
| image digest | `asia-southeast2-docker.pkg.dev/test-youtube-study-space/mypage/mypage-api@sha256:4020ade3cb0b6114b56f41938620bac253e4951f32d3cff472ec28583312c210` |
| system tree | `b484aa740fd4e05c899082d5bc0cfc9deae4b4b1`。以降の制御修正sourceとも同一で、追加image buildなし |
| Frontend source | `38ae9296e7120902f6f7ec76b66508474bd70ea2`、DEV site `test--study-space`の所有projectを照合 |
| Hosting release / version | `1792827165956000` / `67e3543a0a31a77b`（2026-10-24 07:32:45.956 UTC） |
| Frontend実体 | staged 8 filesをlive body SHA256で照合。JSは`index-CEDAJLCZ.js`、CSSは`index-DjiJpEFS.css`。5分更新・route/online/hidden/backoff修正を含む |
| Cache/security | 正確なhash付きJS/CSS/4 fontは`public, max-age=31536000, immutable`。HTML・SPA未知hash・APIはno-store。CSP/nosniffを維持、匿名APIは403 |
| 最終Cloud Run | `mypage-api-00006-xjf` / generation 6 / 観測OFF。history details、Privacy intake、analyticsはOFF |

5分更新・monthly writer/reader・明細と予算のsourceを含むAPI imageは配備したが、既存ユーザーを月次へ切り替えていない。月次activationは所有する合成Userだけに限定し、終了時にそのUserごと削除した。全Bot writer fleetの配備・drainや既存migrationを完了した意味ではない。明細用indexやpolicy/quotaは追加・有効化していない。

## 観測ON/OFF、競合とState

既存Plan/HMAC手順により[観測ON](https://github.com/kani3camp/youtube-study-space/actions/runs/40077157558)でgeneration 5 / `mypage-api-00005-hgk`となった。live更新後にState反映が失敗したため後続を停止し、[新しいState復旧判断](https://github.com/kani3camp/youtube-study-space/actions/runs/40081768487)と[試験前metadata整合](https://github.com/kani3camp/youtube-study-space/actions/runs/40085407554)で12資源のStateとライブを照合してから合成試験を開始した。

実v2のstale etag付き設定不変PATCHは、mask/If-Matchを含む試験でも成功Operationを返した。これを競合拒否の証拠にはしない。[PR #1450](https://github.com/kani3camp/youtube-study-space/pull/1450)で実適用に必要な更新をv1の元`resourceVersion`付きPUTへ限定し、v2ライブ/Plan前提、完全spec・番号付きSecret参照保持、1回だけの送信、競合時停止、結果のv2照合を維持した。ユーザーからDEV限定full-copy PUTとOFFの追加許可を受けている。

合成削除後、現在のReadyを元に新しいOFF Planを作り、[観測OFF](https://github.com/kani3camp/youtube-study-space/actions/runs/40086892890)が成功した。実GETでgeneration 6 / Ready `mypage-api-00006-xjf`、imageと他設定不変を確認。その最新完全copyに元ON時`resourceVersion`を付けた設定不変PUTは実APIで409拒否され、送信前後のetag/generation/spec不変も確認した。これは当該実競合の拒否証跡であり、全ての並行変更パターンを証明するものではない。

一時付与していた1権限`run.services.update`を既存CI Apply SAから解除し、試験用custom roleを削除状態にした。元IAM etagを使って他binding不変を照合し、Cloud Run image/spec/Readyも維持した。GitHub Environmentの既存runtime入力は両方とも`read_metrics_enabled=false`、`read_metrics_from_revision=""`へ復元。[最終metadata同期CI](https://github.com/kani3camp/youtube-study-space/actions/runs/40087126693)のPlan/Apply/State readbackは成功した。2026-10-24 11:25:01 UTCに、AWSプラグインの既存「terraform state」接続から正確なDEV State keyを独立して読み戻した。S3 versionを固定して読み、serial 14・12資源、暗号化、lock不在を確認。既存契約と同じ正規化で、Cloud Runの全設定とUID/generation/observed generation/etag/更新時刻/Ready・created revisionがライブと完全一致した。State本文・bucket/account・credentialを公開していない。

## 予算と後片付け

データ試験は2026-10-24 10:28:17～10:40:53 UTC（約12分36秒）。承認上限は今回US$1、raw作成3000件、作成+削除6000回、document read 20000、index entry 100000、試験約20分。

- raw作成累計2613件。write/delete予約5260回は将来cleanupも含む保守的な予算で、実請求write数ではない。
- document read見積もり17467（driver17357 + 独立cleanup照合10 + 後続通常認証API allowance100）。SDK再試行等を請求書で確定した値ではない。
- Analyzeの実index scan合計2612、事前allowance13100、結果不明のindex operation 0。文書数limitを一般のindex課金上限とみなさない。
- 予約write/deleteをすべて高いwrite単価で計上したFirestore分は約US$0.013。image保存・Run・ログ・Auth等を含む実請求総額の確定ではなく、合計US$1承認を請求書上で照合できたとはしていない。無料枠を使った控除はしない。

各fixtureの差し替えとcleanupは作成時に保持したrevision/hashを使い、conflict時に新revisionへ追従しない。最後の境界fixtureではQAのtyped hashがJST/UTCの表記差で停止した。rawからの旧期待hash、元SAC checkpoint、全原記録revision/hashをread-onlyで再監査し、3時刻のUTC正規化だけを独立レビューした上で全値一致を確認した。内容一致を確認せず所有権を採用するfallbackや無条件削除は行っていない。

cleanup後に別のread-only照合でUser/raw/月次文書/座席不在と元SAC checkpoint不変を確認した。正規ログインで作成されたWebAccount/Authアカウントは削除せず、既存利用者の履歴・moderation記録を試験へ流用していない。

## 認証経路と未完Gate

試験中にChromeのApp Check/reCAPTCHA取得が失敗し、ログアウト後の再ログインが間に合わなかったため、合成raw/月次のHTTP BFF比較は実施していない。Auth/App Checkを無効化したり、browser credentialを取り出してowner SDKへ流用したりしていない。

ユーザーの再ログイン後、正常な未登録・履歴なしの専用アカウントで今日/今週/累計0と直近7日0の通常画面を確認した。Cloud Run request logは2026-10-24 11:06:52.123491 UTC、revision `mypage-api-00006-xjf`、HTTP 200、latency 0.072165407秒で一致した。この単発latencyをCPU・cache miss/hit比較やSLOにはしない。前のreCAPTCHA失敗は再現せず、根本原因を修正済みとはしていない。

App Check token TTLは3600秒。key metricsの同じ日単位windowで前後合計66→66を得たが、反映遅延・日粒度のため評価数0の証拠や1 poll単位の帰属ではない。組織共有の請求評価数・SKU ID・Billing実績は未照合。API pollingを1/5にしただけでApp Check費用を1/5にはしない。

残る実証は、直近window内の月跨ぎと実2月lookup、認証付きraw/月次HTTP比較とcache/CPU、母集団分布とbrowser転送量、請求照合、全writer fleetと既存migration、明細index/共有quotaのlive適用。明細の技術上限50/100件・31日は実装済み候補で、商品としての最終枠や690円Premium発売を決めたものではない。再開条件は既存[Issue #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)で追跡し、この部分完了だけでIssueをcloseしない。