# MyPage real E2E preparation and rollback

2026-10-22のDEV匿名拒否試験とread-only metadata確認は[実測記録](#2026-10-22-devの限定確認)を参照する。実Google OAuth、複数account/回線、S01/S04全層、deploy/rollback rehearsalは未完了。追加のlive操作は対象・account・release対・取得/保存する証跡を先に確定し、[runbook Gate](README.md)後に実施する。local/Emulatorの成功を実Google/Firebase/Hostingの成功へ読み替えない。現行の[server entrypoint](../../../system/cmd/mypage-server/main.go)はkeyless credential、Firestore、OAuth provider、HTTP listenのbootstrapが必要で、sourceやdeploy templateだけではdevelopmentのHosting→Cloud Run経路が存在・稼働する証拠にならない。

## 2026-10-25 追加の0B公開前受入（未実施）

正式構成はFirebase Hosting `/api/**` → Cloud Run Go API + Firebase Auth / Firestoreで、Managed WAF・外部LBなし。[Current Canon 06](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634)、[公開受入 #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)、[monitor auth #1459](https://github.com/kani3camp/youtube-study-space/issues/1459)を参照する。0Aの運営者限定試験中に作業できるが、次の項目が未確認なら0Bを開始しない。

| Gate | 実経路の受入 | 現時点 |
| --- | --- | --- |
| MON-01 Rules | Firebase Client SDKから匿名・通常MyPageユーザーの `seats/member-seats/menu/work-name-trend/config/constants` read/listが拒否、専用monitor UID/claimのみ必要read許可、全client write拒否 | source/Emulator contractあり、DEV/PROD未実証 |
| MON-02 OBS | OBSブラウザソースのInteractログイン、`browserLocalPersistence`、scene切替/OBS・PC再起動/24時間相当稼働、token更新・失効・通知・再ログインを実OBS/CEFで確認 | 未実証 |
| MON-03 cutover | DEV/PRODでRules version・monitor購読・ seat-count API回帰・バックアウト手順を整合。認証不能時に匿名readへ無断rollbackしない | 未実証 |
| COST-01 limits | Run DEV min0/max1・PROD min0/max3をlive readback。requestのtimeout/body/query上限とUID/IP limiter・S01をHostingおよび`run.app`直接で検証 | 部分sourceあり、実経路要確認 |
| COST-02 notify | Billing Budget通知と届け先の実着信、Cloud MonitoringでRun/Firestore reads/429/5xx/instance/YouTube quotaの異常通知 | 未実証 |
| COST-03 cap | Billing alertはhard capでなく、Cloud Run Spend Capは対象範囲・使用可否・他service停止影響を検証後に任意採用。Firestoreには独立した抑止策が必要 | 採否未決 |

上記を完了させるための変更はコード/Rules/クラウド資源を個別reviewし、未承認のProduction deployやOAuth実操作をこの手順の追記だけでは実施しない。

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

## 2026-10-22 DEVの限定確認

2026-10-22 16:08:21〜16:11 JST（07:08:21〜07:11 UTC）に、DEV `test--study-space.web.app` と[固定backend host](../../../infra/mypage-platform/environments/dev/runtime-release.json)へ匿名の少量requestを送り、Cloud Run・Logging・IAMの非機密metadataだけをread-onlyで確認した。実OAuth開始、callback、実account/token、実data更新、rate limit到達、deploy、IAM/ログ設定変更は行っていない。API拒否試験は各8件を2回実施した。初回は検査側が仕様上の固定`message` fieldを除外したためenvelope検査が不合格となり、仕様に合わせた再確認8件がすべて成功した。最初の失敗をsecurity gateのFAILや漏えい検出とは扱わない。

試験前後のCloud Run metadataはgeneration `3`、latest Ready/Created `mypage-api-00003-hcq`、同revisionへのtraffic `100%`で一致した。実revisionのdigestは`runtime-release.json`の`image_digest`と一致し、source参照は同fileの`image_source_sha`とした。Hosting release ID/配信assetの同一性は今回readbackしておらず、完全なFE/BE release対の受入証跡ではない。Go binaryのbuild versionや新image配備をこの確認から推定しない。

| 確認した匿名request | 観測結果 |
| --- | --- |
| Hosting `GET /`, `/privacy`, `/terms`, `/contact`, `/mypage` | 5件とも200、HTML、CSP/X-Frame-Options/Referrer-Policy/nosniffあり。`/mypage`は公開static shellの到達確認のみ |
| Hosting `GET /api/mypage`、App Check/Authなし | 403 `APP_CHECK_REQUIRED` |
| Hosting `GET /api/mypage`、不正な合成App Check | 403 `APP_CHECK_REQUIRED` |
| Hosting `GET /api/mypage`、別Origin | 400 `INVALID_REQUEST` |
| Hosting `POST /api/auth/youtube/start`、同一Origin/空JSON、App Checkなし | 403 `APP_CHECK_REQUIRED`。OAuth transaction/state発行は行わない |
| Hosting `POST /api/auth/session/complete`、同一Origin/空body、App Checkなし | 403 `APP_CHECK_REQUIRED` |
| 直接Run `GET /api/mypage`、通常のbackend Host | 400 `INVALID_REQUEST` |
| 直接Run `GET /api/mypage`、公開Hostを申告するX-Forwarded-Host/https、App Checkなし | 403 `APP_CHECK_REQUIRED` |
| 直接Run `GET /api/mypage`、上記転送headerと別Origin | 400 `INVALID_REQUEST` |

8件のAPI再確認では、bodyが固定の`error.code/message/requestId`だけであり、`message`が`request rejected`、`Cache-Control: private, no-store`、`Set-Cookie`なしであることも検査した。直接Run URLには到達できるため、ネットワーク遮断済みとは扱わない。転送headerは本人確認にはならず、直接経路でもアプリguardが必要。有効App CheckでAuth欠落/不正Authを拒否する401、別UIDの隔離、同意・ServiceAccessControl、実CookieとOAuthの成功/失敗はこの匿名試験では未検証である。

Logging metadataではproject内sinkは`_Default`/`_Required`のみで、`_Default`の`mypage-sensitive-request-url` exclusionが有効、filterは[review済みJSON](../../../infra/mypage-platform/environments/dev/request-log-exclusion.json)と一致した。除外作成時刻2026-10-21 11:32:10.927480464 UTC以降の同filter該当request logは、上限1001件のmetadata件数照会で0件。今回の固有User-Agent/時間窓による通常API request logは16件（400が6件、403が10件）で、queryなしAPIのログは維持されていた。code/state、Cookie、token、request URL、log本文は取得・出力していない。新しいcallback markerも生成していないため、この0件だけでcallback到達や全層S04の成功を宣言しない。

`global`のlog bucketは`_Default`がACTIVE/30日、`_Required`がACTIVE/400日/lockedだった。これは当該bucketの保持設定で、OAuth情報が400日残るという意味ではない。除外前の既存記録の処置、Hosting/CDN、ancestor sink、他region/destination、app/security log、閲覧者・転送先は未検証。sinkは独立にroutingを評価するため、`_Default` exclusionだけでは全層への非保存を保証しない（[Cloud Logging routing](https://docs.cloud.google.com/logging/docs/routing/overview)）。

DEV runtime専用SAにはproject IAMの`roles/datastore.user`が実在した。server SDKはSecurity Rulesで評価されずIAMで認可される（[Firebase server libraries](https://firebase.google.com/docs/firestore/client/libraries)）。通常APIのUID認可とは別に、runtime侵害時には他のFirestore dataにも影響し得る残余リスクがある。不要な操作permissionを減らすcustom roleやdatabase境界の条件を別途評価し、collection単位の隔離ができたとは扱わない。今回IAMは変更していない。

2026-10-22のGitHub read-only inventoryでは、default branchのopen Dependabot alertは103件（Critical 7 / High 41 / Medium 42 / Low 13）、内訳はsystem 1 / monitor 16 / docs 47 / video 14 / menu 21 / CDK 4件だった。MyPageはdefault branchへ未統合のため当該manifest/alertに現れず、安全証明にならない。systemのcompressは今回の修正対象で、他packageの残件は既存Security Issue #864で別途対応する。CodeQLのopen alertは6件（Critical 1 / High 3 / Medium 2）で、monitor/BGMとvideoBGM/common sanitizerが対象、MyPage該当はなかった。修正PRやCI成功だけでこれらを完了/0件に変更しない（[Dependabot](https://github.com/kani3camp/youtube-study-space/security/dependabot)、[Code scanning](https://github.com/kani3camp/youtube-study-space/security/code-scanning)）。

S01（複数の実回線、bucket分離、429/Retry-After、偽装XFF）、全層S04、通常/Brand/複数account間の隔離、完全なrelease対、緊急遮断/復旧rehearsalは引き続き`pending`。0B/一般公開Gateは閉じたままとする。

## 2026-10-22 セキュリティ更新のlocal検証

既存DEVの上記read-only確認と、以下の修正後local検証は別のreleaseとして扱う。Go / Node / dependencyの正確な版は各canonical config / lockfileを正とする。今回の検査入力はGo 1.27.3、Node 24.22.1、Vite 8.3.5、Vitest 4.1.12、Goのcompress 1.18.7 / x/net 0.60.0。3 Dockerfileは修正版builderのdigestを固定し、`GOTOOLCHAIN=local`と`go.mod`とのcompiler一致検査で古いbuilderを自動downloadにより隠すことを防ぐ。

| local検査 | 実施結果 |
| --- | --- |
| MyPage frozen install / check / typecheck / test / build | Node修正版で成功。13 test files / 196 tests |
| MyPage `pnpm audit --audit-level=high --json` | Critical / High / Moderate / Lowすべて0。途中の旧npm audit API timeoutは再実行で解消し、CIで失敗を無視する設定は追加していない |
| Go `go test -shuffle=on ./...` / lint / `I18N_BASELINE=ja go generate ./...` | 成功、意図しない生成差分なし。Go対応のlint版はCI設定を参照。room-image-promptのtest/lintも成功 |
| Go `govulncheck -test ./...` | 到達可能な脆弱性0件。required module内のみのopenpgp `GO-2026-5932`が1件残るが、import/call graphには含まれない |
| MyPage linux/amd64実image / offline nonroot check | buildと設定不足拒否/合成設定受入が成功。抽出binaryのGo 1.27.3、CGO無効、修正版依存を確認 |
| Trivy v0.76.1 / OS + library | 実imageのHigh / Criticalは0、未修正版を一括除外していない。開発依存を含むfrontend lockfile検査でもHigh / Criticalは0で、同じ検査が変更前lockfileのHigh / Criticalを拒否することも確認 |
| CI契約 / routing / actionlint / API契約 / 文書参照 | 19 CI契約test、path routing、変更workflow構文、8 endpoint / fixtures検査、文書リンク検査が成功 |

local image IDは`sha256:3e73f3e7dc9f2f75953e57526e75e8d3abe5516fd942ec6552ddcdee27223776`。これはlocal DockerのIDで、Artifact Registryへ公開したmanifest digestやDEVに配備されたimage digestではない。`-s -w`の実binaryに対するgovulncheck binary modeはsymbolがないためmodule粒度へfallbackし、上記openpgpを検出してexit 3となった。同条件のstripなしbinaryでは到達0件/required moduleのみ1件となる比較を実施した。sourceのimport/call graphとTrivyのHigh/Critical結果を根拠とし、「全moduleの指摘0」とは扱わない。

変更したsourceのGitHub Actions、統合、registry push、DEV更新、実release対での再試験はこのlocal検証の完了とは別条件。Dependabot設定もdefault branchへ反映するまで日次運用開始とは扱わない。既存DEVを修正済みと宣言せず、S01/S04・複数account・復旧・公開前Gateを維持する。

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

## 緊急遮断と復旧のrehearsal（未実施）

既存構成の緊急遮断候補は、対象Cloud Runサービスの公開Invokerをownerが取り消す方法。これによりOAuth/個人APIへの公開経路をサーバー側で閉じ、別配信のHosting static法的ページを維持する想定である。[Cloud Run IAM](https://docs.cloud.google.com/run/docs/securing/managing-access)の前提を確認し、DEVで遮断と復旧を両方実証するまで「停止可能」のGateをPASSにしない。Cloud側IAMの変更は実経路の可用性を変えるため、対象・時間窓・復旧対を記録した個別承認後にのみ実施する。

1. 対象project/region/service、実際のHosting rewrite先・Run URL・trafficをreadbackし、review済みFE/BE対とIAM policyのversion/etagをprivateに保全する。Invoker IAM checkが有効であり、service/project/ancestorの他の公開grantや迂回経路がないか確認する。Check無効化状態では`allUsers`削除だけで遮断できると判断しない。
2. image更新、public apply、Hosting後続release、Bot/外部告知を停止する。ownerがetag保護で当該serviceの`roles/run.invoker`/`allUsers` memberだけを除去し、他のrole/member/conditionを変えずreadbackする。IAM反映後、Hostingと直接RunのOAuth/個人APIが拒否され、OAuth開始/確認/mint/dataが到達不能であることを少量の合成requestで確かめる。Hosting `/privacy`, `/terms`, `/contact`の200と法的本文を確認する。
3. 新規admission拒否は既に実行中のrequest/instanceや発行済みFirebase sessionのdrain/revokeを証明しない。侵害・削除対応では[全runtimeの停止/drain](../support-delete-inventory.md)を追加し、必要なruntime権限隔離・session revokeを別途reviewする。Firestore Rulesの変更をserver権限遮断の代用にしない。
4. 原因修正とreview済みrelease対の確認後、復旧承認を得て同じserviceの元のmemberだけをetag保護で復元する。Hosting/直接Runのnegative probeと承認済み正常経路、同意版、S01/S04を再検証し、元policyとの差分が当該memberだけであることを確認する。一般告知は独立Gateのまま。
5. 一時IAM差分は専用Terraform Stateに対するdriftになるため、停止中にpublic pipelineで再公開・自動修復しない。rehearsalの開始/完了、拒否/静的ページ/復旧の結果、IAM readbackとrelease対のprivate参照を既存release recordへ残す。`destroy`、Secret削除、TTL/DB削除、Auth/App Check/Originの緩和は使わない。
