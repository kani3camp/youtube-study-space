# MyPage real E2E preparation and rollback

ここにあるlive操作は未実行。承認された対象・account・release対・取得/保存する証跡を先に確定し、[runbook Gate](README.md)後に実施する。local/Emulatorの成功を実Google/Firebase/Hostingの成功へ読み替えない。

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
| L01 | Hosting/API/security logsでcallback/query記録 | code/state/token/support queryを保存・通常表示しない。設定全層を確認 | local codeの非出力だけでは証明不可 |
| D01 | 登録済み/未登録/coverage欠損/正常0/取得失敗/partial | unavailableと0/未登録を分離、snapshot/asOf、1000件制限 | aggregation/BFF fixtures |
| D02 | JST日跨ぎ・seat終了/break移動と同時read | double countなし、writer atomicity、cache失効/GeneratedAt | repository/workspaceapp/mypage Emulator |
| M01 | metadata正常更新/上流失敗/正常channel不在/30日境界 | public metadataだけguard付き更新/clear、旧表示が復活しない | metadata mock/Emulator |
| M02 | scheduled cleanup・sink到達・heartbeat/失敗alert | 29日対象、30日超保持なし、成功scan間隔23h以内/job10min以内、alert到達 | cleanup/health tests; scheduler/sink実結線は別 |
| S01 | 受付→fresh support proof（delete/revoke/disclosure）、wrong channel/purpose/replay | request/environment/purpose/channel binding、参照だけ、normal loginに流用しない | support auth/runtime/Emulator |
| S02 | login不能/本人確認不能/期限近い受付 | 公開窓口で返信/個別対応、受付時刻から7日を延長しない | intake運用は別証跡 |
| S03 | D01選択後の横断削除/再実行/復元/遅延writer/cache | 承認対象だけ、全storeとbackup/export/log確認、古いdata復活なし | 選択/実inventory待ち、dry-runは実行証拠でない |
| C01 | GA4 unset/deny/grant/withdraw/他tab、callback/support query | opt-in前request0、過去event再送0、private値0、拒否でもlogin可 | GA4/privacy/runtime tests; real Console/network別 |
| U01 | PC/mobile/320px/keyboard/200%/image/font失敗/bfcache | 読める/操作可、画面外overflowなし、前UID/late response/proofを復活させない | visual/font/privacy/runtime/実bfcache local |
| B01 | 別scopeで実装後、Phase1の !app/!mypage/!page/!my page | 全alias同じ固定public入口/UTM、secret/UIDなし、従来my option不変 | 本PRはruntime機能を追加しない。正本のalias/UTM契約と別実装時のparser/workspaceapp testsで確認 |

false/未実施をPASSへ置換しない。実E2Eごとにscope、対象release対、開始/完了時刻、sanitized evidence参照、結果（pass/fail/pending）を残す。

## Deploy / rollback rehearsal

現行canonはpinTagなし、Backend先行→Frontend後行、必要時Frontend先rollback。schema/responseは前後1世代互換を保つ。以下の操作は承認された環境でのみ実施する。

1. 直前正常なFE/BE対、schema/同意版、Serving revisionとHosting rewrite先を記録する。rollback先を記憶から選ばない。
2. 新Backend + 旧Frontendで6 endpoints/partial/error/同意versionを確認する。壊れる場合は新Frontendを進めない。
3. 新Backend + 新Frontendでmatrixの必須結果を確認する。適用したpolicy版と記録したconsent版を照合する。
4. Frontendを直前正常releaseへ戻し、Hosting経由で新Backendと実通信することを確認する。必要ならBackendも対応する直前revisionへ戻す。
5. 旧正常対に戻ったactual rewrite/revision、監視回復、callback/同意migration windowを確認する。pinTag採用への変更や拒否版の緩和は別review。

rollbackは保存dataの削除やprivacy guard解除を自動的に取り消さない。新規data/schema/consent migrationが後方非互換なら停止して専用復旧手順をreviewする。実ユーザー公開範囲・Bot/外部告知を広げる操作は試験の成功後も別のowner判断。
