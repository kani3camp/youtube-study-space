# Development Runtimeの配備準備

構築時の手順・診断履歴を保持する文書。最新のimage設定は[固定release設定](../environments/dev/runtime-release.json)、配備・実試験の状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。過去の未配備・再確認待ちを現在状態へ読み替えず、公開Gateは対象releaseごとの証跡で確認する。

対象は`test-youtube-study-space` / `asia-southeast2`だけ。Foundationの8資源とRuntime配備を区別し、実ユーザー公開のGateは[release手順](../../../docs/mypage/release/README.md)を維持する。

## コンテナ

### 2026-10-24：既存Stateの遅れを限定して整合する

Goalは、実読したMyPage専用DEV State serial 8（generation 2/image `ef81de63`）と現在live generation 4/image `4020ade3`を、remote writeなしで整合すること。[run 40065942906](https://github.com/kani3camp/youtube-study-space/actions/runs/40065942906)は旧Stateとの差を`runtime-refresh-fields`で拒否し、Applyなし。開始時liveはgeneration 3だったが、Stateはgeneration 2のままだった。Stateの事前照合不足を修正し、観測OFF・合成未実行の状態で後続を停止した。

Invariantsは、固定DEV 12資源・全planned action no-op・全設定比較・unknown/import/replace拒否、native lock、freshな独立Plan/HMAC、保存Planのhash/期限、元S3版・現在liveのUID/etag/Ready/traffic。Non-goalsは再配備、IAM/Secret変更、State手書き/push、一般的な複数世代差の受理、metrics transitionの条件緩和。受入は同じ正規経路でState-only適用後、Stateとliveが一致し、以後のmetrics Planが通常経路を通ること。

`owner-state-recovery.json`は今回だけのsource固定receipt。既存AWS接続で取得したS3 object版・lineage・serialとservice UID、GCPで取得した現在etagをhashへ拘束する。原文Stateやbucket/role IDは公開しない。中間revisionはownerによる3本のReady/config readbackの補助証跡であり、当時のService generation/trafficをそれだけから推測しない。始点は実State、終点はfresh Planと現在live GETで検証する。State上で中間世代を仮想再現しない。

復旧の有効時だけexactなgeneration/revision/image・State版・UIDを要求し、Apply直前と直後にlive GETで全設定とetag等を照合する。競合・別版・別設定・generation 3/5・未完Planは停止し、再採取して自動継続しない。metrics ON/OFFは従来どおり別のfresh Planと元etag条件付きPATCHであり、このreceiptを流用しない。回帰は`test_owner_state_recovery.py`で正常State-only順序、receipt/State/live不一致とconfig変更の拒否を検査する。実適用成功までは復旧済みとしない。

復旧sourceの[実Plan 40072743264](https://github.com/kani3camp/youtube-study-space/actions/runs/40072743264)はState版/世代の照合を通過した後、`metrics-live-config-mismatch`でApply前に停止した。値を公開せず実Stateとliveの全設定を比較し、providerのenv block正規化順とRESTの挿入順だけが異なり、全name/value/Secret参照が一致すると確認した。`match_state`だけで重複・空nameを拒否してenvを名前順に比較し、保存Plan・API mutation・他のlist順序は変更しない。値/参照変更、重複名、args順序変更を回帰で拒否する。新sourceのfresh Planで再検証する。

[復旧run 40073932938](https://github.com/kani3camp/youtube-study-space/actions/runs/40073932938)は独立Plan/HMAC・保存Planの適用・live照合まで成功。独立AWS readbackでも12資源・serial 9・generation 4/image `4020ade3`、元service UID不変を確認した。続く[metrics ON試行40075166738](https://github.com/kani3camp/youtube-study-space/actions/runs/40075166738)はPlan成功後にAPI呼出で停止し、liveはgeneration 4・OFFのまま。sourceの同じwire形をCloud Run公式`validateOnly=true`で400として再現した。providerが補う空の`value`/`value_source`を両方JSON oneofへ入れていたため、選択した片方だけを送るencoderへ局所修正し、保存なし検証200を確認。空のplain文字列は保持し、未知field・競合するoneof・重複名は拒否する。

別にIAMを読み、既存Apply SAが`run.services.update`を持たないことを確認した。利用者から、今回のUS$1以内のDEV試験のため、同permission 1個だけの一時custom roleを固定`mypage-api`のApply SAに限定して付与・試験後解除する追加承認を得た。これはCloud IAM上ではサービス更新権限であり、metrics env以外の拒否はレビュー済みPlan/conditional PATCH guardが担う。権限追加を一般許可に読み替えず、既存binding・Invoker・credential・Productionを変更しない。IAM memberのetagだけのState refreshを通常経路で完了してから、freshなON/OFF Planを行う。終了後はbinding/一時roleを解除し、そのIAM metadataのState整合も確認する。実試験成功・解除完了は別のrelease証跡に残す。

`system/Dockerfile.mypage`は既存のdigest固定Go / distrolessイメージを使用し、Cloud Run用のlinux/amd64バイナリをビルドする。CGOなし、timezone dataをバイナリへ含め、nonrootで起動する。Secretやローカル環境ファイルをイメージへ渡さない。

```sh
docker build --platform linux/amd64 --file system/Dockerfile.mypage --tag mypage-api:ci system
bash system/scripts/check-mypage-image.sh mypage-api:ci
```

通常のSystem CIで実イメージをビルドし、ネットワーク無効・read-only filesystemで`--check-config`を実行する。設定なしの拒否、合成設定の受理、nonrootを確認する。この検証はADC、Secret値、実Google OAuth、Cloud Run起動や公開Readyを証明しない。CIはimage push / deploy / cloud認証を行わない。

## 実環境の現在地

### DEV認証情報の接続

2026-10-21に利用者の実行時確認を受け、MyPage専用のOAuth Web client（固定Hosting callback）とYouTube APIだけに制限したserver keyを作成した。同じDEV projectの既存3 Secretへ値を保存し、すべてversion `1` / `ENABLED`をmetadata-onlyの独立readbackで確認した。値をGitHub/チャット/CIへ渡していない。API Keys / YouTube APIの有効化とSecret version lifecycleはoperator管理であり、既存9資源Stateへpayloadを追加しない。

Plan/Applyの両専用Environmentへ`MYPAGE_RUNTIME_SETTINGS_JSON`を接続した。中身は公開設定、draft同意版、image digestと番号固定Secret参照だけで、payloadは含めない。実OAuth/E2Eや文面公開承認を完了した意味ではない。

Runtime publicの実Plan [39546080459](https://github.com/kani3camp/youtube-study-space/actions/runs/39546080459)と[39547114678](https://github.com/kani3camp/youtube-study-space/actions/runs/39547114678)は、native Plan完了前の汎用`check=contract`で停止し、Applyなし。同じsourceのimage tag/digestと3個のENABLEDなversion metadataはoperatorが独立照合した。切り分け用の空local Stateでは同じsource/configのnative Plan生成が成功したが、実StateとCI identityでのPlan成功を代替しない。

実Plan経路はRuntime入力照合とTerraform実行のphaseを区別し、source image digest不一致・version無効・既知のnative失敗を固定source labelで報告する。native stdout/stderr、command引数、rejected value、未知のpermission名は出力しない。診断の追加でadmission、権限、独立Plan/HMAC、cleanupの許可範囲を広げない。

初回[Runtime private Plan 39543529630](https://github.com/kani3camp/youtube-study-space/actions/runs/39543529630)は`registry-refresh-empty-labels-only`で停止し、Applyなし。独立State serial=3のRegistry作成時刻と更新時刻は同一だったが、GCPの更新時刻は検証済みimage push時刻へ進んでいた。guardは同一固定Registryの予定no-opで、空labels表現差とcomputed `update_time`の単調なRFC3339 UTC更新だけを受理する。nanosecond精度の逆行、他field、unknown、予定writeは拒否し、全driftをHMACへ保持する。修正後は新source imageとnative Planで再確認する。

一時bootstrap権限の解除も共有IAM policyのetagを変えるため、Runtimeの固定Firestore memberとpublic stageの固定invoker memberには、Foundationと同じ「同一managed member・予定no-op・非空etag以外の全field一致」の判定を使う。実際のrole/member/condition変更、duplicate、unknown、予定write、Cloud Run本体やprivate stageへのpublic member追加は拒否する。この2 memberのcleanup後live確認は実Runtime Apply後に行う。追加の権限付与や実Applyをこのsource変更で行うものではない。

### Bootstrap metadataの追従

Runtime準備では、project上のmetadata-only custom role（Registry tag、Cloud Run/operation、project IAM、Secret version metadataのget）をPlan/Applyへ、download/uploadだけのcustom roleを専用RegistryのApplyへ、actAsを専用runtime SAのApplyへ限定した。Secret payload、Cloud Run create、project IAM editorは付与していない。専用SAへのactAs追加で共有IAM policyのetagが変わることをGCP readbackで確認した。Registry IAM変更によるrepository updateTime変更はなかった。

IAM memberのreadは共有policyのetagを返すため、当該4 memberの予定操作がno-opで、before/afterが非空etag以外は完全一致するrefreshだけを許可する。member/role/condition/id/Secret/projectの変更、unknown、予定write、別resourceは拒否する。生driftは全Plan HMACへ保持し、operator管理のbinding全体をTerraform-ownedに読み替えない。旧guardの[実Plan run 39527611450](https://github.com/kani3camp/youtube-study-space/actions/runs/39527611450)はmetadata-refresh-addressで停止した（Applyなし）。修正後の実Planは別runで再検証する。

etag対応後の[実Plan run 39528628841](https://github.com/kani3camp/youtube-study-space/actions/runs/39528628841)もmetadata-refresh-addressで停止し、Applyは行っていない。独立したState readbackでserial=3、専用Registryのlabels=nullを確認した。Registryについても、固定9資源契約に一致する同一resourceのlabels=null→{}だけを予定no-op時に受理する。timestamp、tag policy、scanning、他default、非空label、unknownや予定writeは引き続き拒否する。この限定対応が当該実Planの停止原因を解消するかは、新runでのnative Plan成功まで未確認とする。

### イメージとRuntimeの実行順序

Registryの実適用後、同じ専用reusable workflowで`image-build`を明示dispatchする。exact actor/ref/workflow/SHAのOIDCを維持し、Apply identityに当該Registry単位のupload権限だけを別途接続する。image jobはAWS認証もTerraform/State操作も行わない。digest固定baseからlinux/amd64 imageを作り、nonroot・network無効の設定検査後、40桁source SHAのimmutable tagへpushし、local RepoDigestとRegistry tagのdigestを照合する。既存tagの上書きは拒否する。

Runtime初回は`runtime-private-plan` / `runtime-private-apply`で11資源（Registry段階9 + 当該SAのFirestore member + Cloud Run 1）だけを許可する。その後、Hosting経路へ接続する段階で`runtime-public-plan` / `runtime-public-apply`を使い、当該serviceだけの`allUsers` invoker memberを追加した12資源に進む。初回create/no-opだけを受理し、update/delete/replace/import、別principal、別Cloud Run/Registry、未知のenvや生Secret値を拒否する。修正revisionへの更新は、この初回guardを無制限に緩めず別のreview済みtransitionで扱う。

両Environmentの追加secret `MYPAGE_RUNTIME_SETTINGS_JSON`へ、`runtime_settings`と同じkeyのJSON objectを設定する。含めるのはimage digest、project番号/Web app ID、固定origin、同意版、3個の番号固定Secret version、任意の確認済みhistory coverageだけ。OAuth client secret/API key/tokenは含めない。重複key、未知field、別Registry/origin、`latest`をpreflightで拒否する。project番号は実WIF providerと一致させ、Planでは当該SHAのimmutable tagと3個のENABLEDなSecret version metadataを照合する。

image-buildとRuntime Plan/Applyのsource SHAは同一とする。間にcodeを変えた場合は、新SHAでimageを作り直してdigest設定を更新する。前後1世代互換やrollbackは[実経路E2E](../../../docs/mypage/release/e2e.md)で別途確認し、初回guardの成功だけで成立と扱わない。

Cloud RunはGen2、service/revisionの両方でDev max=1/min=0、CPU 1/512Mi、request-based billing、startup boostなし、TCP startup probe、固定runtime SAを使う。Cloud Run側のInvoker IAM checkは無効化せず、アプリのAuth/App Check/Origin/同意/ServiceAccessControlを維持する。Firestore `roles/datastore.user`はcollection限定ではなくDEV projectのDB全体へ及ぶため、その付与は実target/権限レビューで明示する。必要なCIのrun/create/actAs/当該IAM member変更権限も、Foundation/Registryのmetadata権限と区別して用意する。

### 専用Registryの段階

`CI`の明示dispatch `mypage_foundation=true` / `mypage_foundation_mode=registry-plan`または`registry-apply`で、同じ独立DEV Stateを使ってFoundation8資源と専用Registry1資源だけをPlanする。既存のexact actor/ref/workflow/SHA、2 identityの独立Plan照合、HMAC、native lock、private raw evidence、保存Plan適用の契約を維持する。旧4/8資源modeの許可範囲は増やさない。

Registryは固定project/region/ID、DOCKER、STANDARD_REPOSITORY、immutable tags、cleanup dry-run・自動削除なし。guardは9資源のcreate/no-opだけを許可し、既存の同名RepositoryがState外にあればcreateを拒否する。source追加だけでは実Registry・image・Cloud Runを作成しない。RegistryをStateへ追加した後は、旧foundation段階へ戻さず`registry-plan`でno-opを検証する。

実適用前にDEVの既存SAへ必要最小限のRegistry metadata readを追加し、Applyだけに初回Repository createを一時付与する。`artifactregistry.repositories.get`の読取りと`artifactregistry.repositories.create`はmetadata作成権限であり、upload、delete、repository IAM、project IAM、Secret payloadは含めない。create grantは1資源の作成とreadback後に解除する。image pushに必要な資源単位権限は、別の実build/deploy接続時に確認する。既存Foundation bootstrap roleへSecret値・Cloud Run・project IAMを混在させない。

2026-10-21 JSTにCloud Shellを再接続し、GCPからFoundationのIAMを独立に再取得した。runtime自身のsigner、3 Secret単位のAccessor、bootstrap用の資源単位editor、Plan/Apply SAのproject metadata-read-onlyとWIF memberが想定どおり。project create grantなし、3 Secretのversionなし、3 SAのuser-managed keyなしを確認した。DEV Cloud Runは0件、同regionのArtifact Registryは既存`gcf-artifacts`のみ。

次の実配備では専用Artifact Registryとdigest固定image、実Firebase app/origin、番号固定Secret version、Runtime用typed Plan/Applyを接続する。既存Functionsのrepositoryやimport-only Stateを流用しない。Google Auth Platformには自動作成の既存Web clientがあるが、MyPage用clientとして採用・変更していない。

実際のServing revision / image digest / Hosting release / 同意版 / 試験結果を[release record](../../../docs/mypage/release/release-record.example.json)へ一対で記録する。OAuth code/tokenや生ログはpublic PR/CI artifactへ残さない。
