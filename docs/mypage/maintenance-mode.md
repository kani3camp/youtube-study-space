# MyPage maintenance mode v1 runbook

この文書は、MyPageだけを安全に停止・再開するv1の実装・運用契約である。状態は`NORMAL`と`ACTIVE`の2つだけで、予告状態、予約切替、自動解除はない。画面に終了予定を表示できるが、時刻を過ぎても状態は変わらない。

## 状態とデータ境界

各Firebase projectの`mypage-runtime-control/current`を唯一の制御文書とする。`environment`、単調増加する`revision`、`state`、`updatedAt`、固定`messageTemplate=MYPAGE_MAINTENANCE_V1`、固定選択の`reasonCategory`を必須とし、`estimatedEndAt`は`ACTIVE`時だけ任意で保持する。文書の欠損、余分なfield、型不正、環境不一致、読取失敗はすべてfail-closedであり、`NORMAL`とはみなさない。

遷移と`mypage-maintenance-events/{actionRef}`の監査eventは同一Firestore transactionで確定する。更新には現在のrevisionが必要で、競合した古いworkflowは失敗する。`NORMAL → ACTIVE`と`ACTIVE → NORMAL`に加え、`ACTIVE`中の終了見込み変更だけは`ACTIVE → ACTIVE`のmetadata更新として新revisionを発行する。終了見込みが同一の純粋なno-opは拒否する。通常経路の監査eventには`authority=GITHUB_ACTIONS`、immutableなrepository/owner/actor ID、triggering actor、run ID/attempt、Git ref、commit SHA、workflow ref、実Cloud Run execution、前後state/revision・終了見込み、固定reasonを残す。break-glassは別の`authority=GCP_BREAK_GLASS`とし、GitHub provenanceを偽装せず、実execution、承認参照、Cloud Audit Logと照合するclaimed operatorを残す。token、URL、OAuth code、cookie、credential、自由記述は保存・出力しない。

初期文書はTerraformで暗黙作成しない。API serving imageとは別の固定`maintenance_operator_image_digest`と、その不変tagを作成した`maintenance_operator_image_source_sha`を使い、未配備環境ではまず`maintenance_operator` stageでJob/WIF/IAMだけを構築する（このstageはCloud Run APIを作成・更新しない）。既に`runtime_public`がStateにある環境では、専用`maintenance_upgrade` stageだけを使い、既存API・Firestore member・public Invokerをすべてno-opに保ったままmaintenance資源だけを追加する。通常の`maintenance_operator`へ戻して既存runtimeを削除するPlanは許可しない。環境ごとにworkflowから`state=NORMAL`、`expected_revision=0`、`reason_category=INITIALIZATION`を一度だけ実行し、authoritative readbackを確認する。その後のruntime plan/applyと通常Backend releaseは、引数上書き不可・Firestore読取専用identityの固定`mypage-maintenance-bootstrap-check` Jobが厳密で初期化済みのcontrolを確認できなければ、serving mutation前に停止する。`NORMAL`と`ACTIVE`はいずれもvalidで、メンテナンス中の復旧deploy/rollbackを妨げない。runtime stageはmaintenance資源の新規作成も拒否する。これにより「operator構築または既存runtimeの安全なupgrade → 初期NORMAL → Backend guard → Frontend」の順序を機械的に固定する。

## 停止対象と例外

`ACTIVE`では`GET /api/mypage`、明細、通常OAuth start/callback/channel/confirm、custom token発行、session complete、account metadata更新を`503 MAINTENANCE_ACTIVE`で拒否する。プロセスキャッシュは最大5秒だが、token発行、transaction内の個人データwrite、aggregate cache publish、個人レスポンス返却の直前には制御文書を直接再読する。通常OAuth transactionと各writeは開始時のmaintenance revisionにも束縛し、処理中に`ACTIVE → NORMAL`まで完了していても古いrequestを復活させない。切替前に始まった処理から切替後に個人データやtokenが返らないことを優先する。

公開トップ、プライバシー、利用規約、お問い合わせ、本人確認済みprivacy request受付・状態確認、目的がserver-side support requestに束縛されたsupport OAuthは利用可能である。clientがsupport目的や対象channelを自己申告して例外になることはない。

`GET /api/public/mypage-status`はsame-origin、認証/App Check不要、`no-store`で、`state`、`updatedAt`、任意`estimatedEndAt`、固定message templateだけを返す。Frontendは取得失敗を`UNKNOWN`として安全側のmaintenance画面に留まり、`NORMAL`を推測しない。

## GitHub Actionsによる手動切替

workflowは`MyPage Maintenance Mode`（`.github/workflows/mypage-maintenance.yml`）。DEVは`integration/mypage-canon-20261006`、PRODは`main`の、レビュー済みexact SHAからだけ実行できる。rerun、fork、pull request、別ref、repository/owner/actor ID不一致、actorとtriggering actor不一致は認証前に拒否する。

1. 対象環境の直前の成功run artifactでcurrent revisionを確認する。公開statusはrevisionを出さないため、90日保持されるbounded transition receiptを使う。
2. `environment`、`NORMAL|ACTIVE`、`expected_revision`、固定`reason_category`、exact `expected_sha`を入力する。`ACTIVE`だけ14日以内のRFC3339 `estimated_end_at`を任意入力でき、認証前にcanonical UTCかつFirestoreのmicrosecond精度へ正規化される。`ACTIVE`開始は`PLANNED_MAINTENANCE|DATA_MIGRATION|SECURITY_MAINTENANCE`、解除は`RECOVERY`だけを使う。保守延長・短縮・見込み未定への変更は、current revisionと同じ`reason_category`を使って`ACTIVE`のまま新しいrunを開始する。
3. PRODでは`PRODUCTION ACTIVE`または`PRODUCTION NORMAL`をexact入力し、GitHub Environment approvalを通す。
4. 初期`NORMAL`以外では、認証前にcanonical originの厳密な公開statusとHTML内の固定`MYPAGE_MAINTENANCE_V1` capability markerを読み、Backend guardとFrontend UXが両方配備済みであること、遷移前state/終了見込みのexact値を確認する。未配備・不明ならFirestoreを変更しない。
5. workflowが専用WIFで`mypage-maintenance-invoker`へ連携し、特定Cloud Run Jobだけを起動する。GitHub側identityにはFirestore権限がない。
6. Jobは認証前に固定した遷移前state/終了見込みも含め、最初にnon-mutating previewでcurrent state/revisionと遷移可否を確認し、次のexecutionでCAS・監査append・authoritative readbackを実行し、最後のnon-mutating verifyでcontrolとaudit eventの前後値がexactに一致することを確認する。
7. 初期`NORMAL`以外はsourceで環境ごとに固定したcanonical HTTPS origin（DEVは`https://test--study-space.web.app`、PRODは`https://moku.work`）の公開statusが、必須timestampを含む厳密schemaで指定stateとexact `estimated_end_at`（未指定ならfield非存在）を返し、公開トップ、MyPage shell、Privacy、Terms、Contactが応答することを確認する。3段のJobとHTTP postconditionが成功したときだけ、revision/state/actionRefを含むreceipt artifactを保存する。初期`NORMAL`はAPI未配備を許すためHTTP確認を`not-yet-deployed`として分離する。

GitHub Environment `mypage-maintenance-development` / `mypage-maintenance-production`には、対応環境のTerraform outputから次を設定する。

- `MYPAGE_MAINTENANCE_WIF_PROVIDER`
- `MYPAGE_MAINTENANCE_SERVICE_ACCOUNT`（`mypage-maintenance-invoker@<project>.iam.gserviceaccount.com`）

WIF providerはsource固定の対象project numberを含むexact resource nameと一致しなければ、認証前に拒否する。attribute conditionで参照するGitHub OIDC claimはすべてprovider mappingにも明示し、immutable numeric owner/repository/actor ID、workflow/ref/SHA、environment、初回attemptを束縛する。公開originもsource固定であり、GitHub Environment variableから別環境や任意hostへ差し替えない。

Terraformでは`runtime_settings.maintenance_operator_enabled=true`とAPI用digestから分離した`maintenance_operator_image_digest` / `maintenance_operator_image_source_sha`を、`maintenance_operator`以降のstageで必須にする。source SHAは実行中commitのancestorであり、Artifact Registryの同SHA tagがoperator digestを指すことをPlan/Applyの双方で検証する。通常のTerraform identity、MyPage API runtime identity、backend deployerをmaintenance workflowへ流用しない。独立した承認者がいる運用体制ではPROD Environmentにrequired reviewerを設定する。単独運用では自己承認required reviewerを要求せず、exact confirmation、`main` / exact SHA、immutable owner/actor ID、初回runだけという機械的制約を維持する。

## GitHub Actions障害時のbreak-glass

GitHub Actionsの障害だけを理由にFirestore Consoleや`gcloud firestore`でcontrol/auditを直接変更しない。通常Jobとは別の固定`mypage-maintenance-breakglass` command/Jobから、同一CAS、同一transactional audit、同一verifyを使う。authorityは各binaryに固定され、引数や環境変数では選べない。これは通常手順ではなく、インシデント責任者が承認参照と対象環境・state・revision・review済みoperator source SHAを記録した場合だけ行う。

1. 対象Jobの現行image digest/source SHAとcurrent state/revisionをread-onlyで確認する。承認参照は`INCIDENT_123`形式、operatorは実GCP principalのemailを使う。
2. 管理者はそのprincipalだけに、対象環境の`mypage-maintenance-breakglass` Job資源上で、短い有効期限条件付き`roles/run.jobsExecutorWithOverrides`を一時付与する。通常`mypage-maintenance-operator` Job、project全体、Firestore、Terraform SA、通常backend deployerへは付与しない。
3. `gcloud run jobs execute mypage-maintenance-breakglass --wait --quiet --args=...`で、`--break-glass-approval-ref`、`--claimed-operator`、exact project/environment/state/expected revision、`--expected-previous-state`、任意の`--expected-previous-estimated-end-at`、reason/action ref/ref/source SHAを渡し、`preview`、`apply`、`verify`を別executionでこの順に実行する。`--authority`およびGitHub repository/run/workflow fieldsは渡せず、渡した実行はFirestore接続前に拒否される。
4. `verify`成功後、公開statusと例外routeを通常workflowと同じ条件で確認し、3 execution ID、control/audit readback、Cloud Audit Logのinitiating principal、承認参照をインシデント記録へ保存する。claimed operatorとAudit Log principalが違えば失敗として扱う。
5. 一時Job IAM bindingを直ちに削除し、readbackで消失を確認する。途中失敗時も自動再試行や同一actionRef再利用をせず、current revision/audit/Audit Logを再確認する。

具体的な引数列はoperatorの`--help`から組み立てず、このrunbookの必須fieldを二者確認する。`actionRef`は承認参照・execution目的・環境・stateからオフラインで生成した64桁hexとし、tokenや説明文を含めない。

## 復旧と失敗時

- CAS不一致: 状態を上書きせず、新revisionを確認して意図を再評価し、新しいworkflow runを開始する。rerunは使わない。
- Job失敗・readback失敗: 成功とみなさない。公開statusと監査eventを読み取り、状態が不明ならfail-closedを維持する。
- 制御文書破損: 手作業で補修せず、原因とcurrent documentを限定的に確認してから、review済みの復旧変更または専用operator拡張を行う。通常APIを一時的にfail-openへ変更しない。
- 解除: `state=NORMAL`、current revision、`reason_category=RECOVERY`で新しいrunを開始する。終了予定は付けない。公開statusが`NORMAL`になり、`/maintenance`の再読み込みで`/`へ置換遷移することを確認する。
- rollback: code rollbackと運用state rollbackは別操作である。codeを戻してもFirestore stateを暗黙変更しない。必要なら安全なcodeを配備した後、別の承認済みworkflowで`NORMAL`へ遷移する。

live deploy、Terraform apply、GitHub Environment/secret設定、初期`NORMAL`作成、実環境の開始・解除は、個別に明示承認された運用作業であり、source mergeだけでは実施済みと扱わない。
