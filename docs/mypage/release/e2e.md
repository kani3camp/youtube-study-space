# MyPage real E2E preparation and rollback

ここにあるlive操作は未実行。承認された対象・account・release対・取得/保存する証跡を先に確定し、[runbook Gate](README.md)後に実施する。local/Emulatorの成功を実Google/Firebase/Hostingの成功へ読み替えない。現行の[server entrypoint](../../../system/cmd/mypage-server/main.go)はkeyless credential、Firestore、OAuth provider、HTTP listenのbootstrapが必要で、sourceやdeploy templateだけではdevelopmentのHosting→Cloud Run経路が存在・稼働する証拠にならない。今回cloud read、deploy、IAM変更、実OAuth、実経路試験は行っていない。

## 実施前のrecord

[template](release-record.example.json)へenvironment/project/domain、frontend SHA/Hosting release、backend digest/Cloud Run revision、rewrite先/pinTag=false、schema/同意版、承認参照、試験scopeを記録する。secret、token、OAuth code/state、supportChallenge/proofRef、channel UID、作業内容をpublic record/CI logへ保存しない。raw HARの共有やpublic artifact保存は行わず、必要なprivate証跡だけをredactして結果/時刻へ結び付ける。

テストaccountは運営者が明示したものを使う。実dataの変更を伴うlogout/revoke/deleteは、その操作と対象を別に承認する。未登録や欠損を作るために実DBを直接壊さない。必要な状態が実accountで用意できなければ、当該実試験をpendingとしlocal代替のscopeを記録する。

## 実経路の受入matrix

| ID | 実施・観測 | 受入条件 | local準備の根拠 |
| --- | --- | --- | --- |
| P01 | anonymous `/`, `/privacy`, `/terms`, `/contact` とAPI前同意 | 公開pathへlogin不要、窓口/本文版一致、draft placeholder解消は承認後のみ | public policy tests / privacy-qa |
| A01 | 同意→start→Google→callback→channel confirm→session complete→MyPage | youtube.readonlyのみ、確認前mintなし、uidは確認channel、登録済み記録のみ | server integration / runtime-qa |
| A02 | scope拒否/callback失敗/confirm cancel、期限切れ/replay | token永続保存なし、説明と再開始、誤mint/遅延復活なし | provider/auth/HTTP tests |
| A03 | 通常account/Brand Account/複数channel、意図とdefaultが違う | 誤channelを確定しない。再試行手順とGoogle側選択挙動を記録 | real Google試験は代替不可 |
| A04 | Hosting経由の__session/callback、Chrome/Safari/mobile | HttpOnly/Secure/Lax/Path=/、Hosting転送、fixed Host、callback完了 | origin/cookie tests; actual Hostは別証跡 |
| A05 | App Check欠落/別app/失効、Auth欠落/旧google provider/別project | guardをdata read/cacheより先に拒否。App Check debug tokenなし | signed synthetic RSA / Firebase boundary tests |
| A06 | UID差替え/extra query/別Origin/duplicate cookie、parallel callback/confirm | IDOR/Origin拒否、claim/consume/mint一回、公開raw errorなし | HTTP origin/auth atomic tests |
| L01 / Canon S04 | Hosting/API/security logsでcallback/query記録 | code/state/Cookie/token/support queryを不要に保存・通常表示しない。platform/app全層を確認 | local codeの非出力だけでは証明不可 |
| Canon S01 | development Hosting→Cloud RunでIP識別・OAuth開始の429 | 同一/別回線を分離、XFF偽装でbucket変更なし、他利用者へ波及なし、Retry-After有効 | process内limiter/RemoteAddrの合成テストだけでは証明不可 |
| D01 | 登録済み/未登録/coverage欠損/正常0/取得失敗/partial | unavailableと0/未登録を分離、snapshot/asOf、1000件制限 | aggregation/BFF fixtures |
| D02 | JST日跨ぎ・seat終了/break移動と同時read | double countなし、writer atomicity、cache失効/GeneratedAt | repository/workspaceapp/mypage Emulator |
| M01 | metadata正常更新/上流失敗/正常channel不在/30日境界 | public metadataだけguard付き更新/clear、旧表示が復活しない | metadata mock/Emulator |
| M02 | scheduled cleanup・sink到達・heartbeat/失敗alert | 29日対象、30日超保持なし、成功scan間隔23h以内/job10min以内、alert到達 | cleanup/health tests; scheduler/sink実結線は別 |
| SUP01 | 人的窓口で受付→依頼に束縛したfresh support proof（delete/revoke/disclosure）、wrong channel/purpose/replay | request/environment/purpose/channel binding、参照だけ、normal loginに流用しない。アプリ内受付UIは不要 | support auth/runtime/Emulatorはsource証拠のみ |
| SUP02 | sessionなし/本人確認不能/期限近い人的受付 | 返信できる公開窓口で個別対応、受付時刻から7暦日を延長しない。新bearer credentialなし | 窓口・担当・実deliveryは未確定 |
| SUP03 | D01選択後の横断削除/再実行/復元/遅延writer/cache | 承認対象だけ、全storeとbackup/export/log確認、古いdata復活なし、完了通知 | live adapter/実inventory待ち、dry-runは実行証拠でない |
| C01 | GA4 unset/deny/grant/withdraw/他tab、callback/support query | opt-in前request0、過去event再送0、private値0、拒否でもlogin可 | GA4/privacy/runtime tests; real Console/network別 |
| U01 | PC/mobile/320px/keyboard/200%/image/font失敗/bfcache | 読める/操作可、画面外overflowなし、前UID/late response/proofを復活させない | visual/font/privacy/runtime/実bfcache local |
| B01 | 別scopeで実装後、Phase1の !app/!mypage/!page/!my page | 全alias同じ固定public入口/UTM、secret/UIDなし、従来my option不変 | 本PRはruntime機能を追加しない。正本のalias/UTM契約と別実装時のparser/workspaceapp testsで確認 |

false/未実施をPASSへ置換しない。実E2Eごとにscope、対象release対、開始/完了時刻、sanitized evidence参照、結果（pass/fail/pending）を残す。

## Canon S01/S04 実経路セキュリティ検証

以下は**将来の承認済みdevelopment実経路**に対する最小試験手順であり、今すぐ実行可能とは限らない。まずowner/infra担当のprivate inventoryで、対象Hosting releaseとAPI rewrite先、Cloud Run service/image digest/実際にtrafficを受けるrevision、`mypage-server`のkeyless/Firestore/provider/HTTP bootstrap、dev URLとログの閲覧境界を確認する。欠ける経路・bootstrap・権限は`pending`として別の実装/配備・承認へ戻す。sourceの`RemoteAddr`利用、Hosting候補、CI成功だけでこの前提をPASSにしない。実値・実account・実ログ本文を公開recordへ写さない。

1. **release対を固定する。** [record](release-record.example.json)に承認済みdevelopmentのHosting release、rewrite、Cloud Run revision/image digest、FE/BE SHA、試験時間窓、rollback対をprivate referenceで固定する。試験中にrevisionが変われば結果を混ぜず再開始する。
2. **S01: 実IPと429。** 承認された同一回線の2つの合成clientと、別回線の合成clientから、負荷を限定してOAuth開始と保護対象requestを試す。`RemoteAddr`と採用bucketの意味をprivate診断で確かめ、同一回線の429が別回線/利用者へ波及しないか、`Retry-After`が正で待機後に回復するかを記録する。単なる`X-Forwarded-For`変更を別回線の代用にしない。同じ送信元から偽装XFFを複数値に変えても制限を迂回・他者のbucketを消費できないことを確認する。現行codeは既定で`RemoteAddr`を使い、XFFを直接信用しない。実proxy chainが不明、実IPを安全に観測できない、他者へ429が波及する場合はFAIL/PENDINGにし、確認された局所修正だけ別reviewする。
3. **S04: callback/log全層。** 承認済みの合成`code`/`state`/`__session` markerでcallbackのHosting→rewrite→Cloud Run転送、直後のcodeなしredirectを確認する。platform request log、利用中のCDN/Hosting access log、Cloud Run request/access log、application/security logと閲覧者・保持設定をprivateに照合し、query・Cookie・token・support参照が不要に記録/閲覧/長期保持されないことを確認する。合成markerで通らない実OAuth部分は承認済み検証accountで別途閉じる。アプリ側redactionだけでPASSにせず、閲覧不能な層もPASSにしない。raw URL/HAR/log/markerをPR・CI artifact・公開recordへ置かず、各層の結果・時刻・redactedな証跡参照だけ残す。
4. **結果の扱い。** S01/S04のpass/fail/pendingと局所修正の根拠をrelease対へ結び付ける。漏洩または誤った429波及があれば0B Gateを閉じ、必要な範囲だけ修正して同じ実経路で再検証する。Redis、WAF、分散limiter、先行ログ基盤の追加を試験の前提にしない。production公開前にも対象release対でログとrate limitの同等の確認を行う。

## Deploy / rollback rehearsal

現行canonはpinTagなし、Backend先行→Frontend後行、必要時Frontend先rollback。schema/responseは前後1世代互換を保つ。以下の操作は承認された環境でのみ実施する。

1. 直前正常なFE/BE対、schema/同意版、Serving revisionとHosting rewrite先を記録する。rollback先を記憶から選ばない。
2. 新Backend + 旧Frontendで6 endpoints/partial/error/同意versionを確認する。壊れる場合は新Frontendを進めない。
3. 新Backend + 新Frontendでmatrixの必須結果を確認する。適用したpolicy版と記録したconsent版を照合する。
4. Frontendを直前正常releaseへ戻し、Hosting経由で新Backendと実通信することを確認する。必要ならBackendも対応する直前revisionへ戻す。
5. 旧正常対に戻ったactual rewrite/revision、監視回復、callback/同意migration windowを確認する。pinTag採用への変更や拒否版の緩和は別review。

rollbackは保存dataの削除やprivacy guard解除を自動的に取り消さない。新規data/schema/consent migrationが後方非互換なら停止して専用復旧手順をreviewする。実ユーザー公開範囲・Bot/外部告知を広げる操作は試験の成功後も別のowner判断。
