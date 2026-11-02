# DEV Planの構成参照権限

CoreとMyPageのDEV Planは、[静的manifest](../policies/plan-metadata-read.json)を正本とする同一のproject custom roleを使用する。対象は `test-youtube-study-space` の `terraform-dev-normal-plan` と `mypage-foundation-plan` だけで、SA・WIF・Stateは別々のままにする。同じSAを使う手動DEV Planにも参照範囲が及ぶ。

manifestのfamilyごとに、明示permission、APIの返却内容、一次資料、設定に秘密値を置かないための前提を保存する。API・predefined roleの追加を自動追従せず、未分類permissionは拒否する。使用していないサービスも含むが、APIの有効化は行わない。請求、Secret payload、業務データ本文、credential発行、sign/decrypt、writeは付与しない。Secret metadataと既存AWS State読取りは別の扱いであり、State/lock policyはこのツールでは変更しない。

Cloud SQL instanceのget/listは、外部primary接続設定のpassword/clientKeyについて全返却形の非公開保証が確定していないため初期対象から除外する。GKEはcluster構成だけで、getCredentialsやKubernetes APIのSecret/Pod/ConfigMap参照を許可しない。Firestoreは `datastore.databases.getMetadata` を用い、transaction操作の `datastore.databases.get` を含めない。

VM metadata、startup script、Run/Functionsの環境変数、Schedulerのbody/header、Pub/Subのpush URL、BigQueryのview/routine定義、監視の設定には平文秘密値を埋め込まない。構成参照permissionの許可は、そのAPIに誤って格納された秘密値を不可視にする保証ではない。返却値をIAMで分離できず既知の秘密値がある場合は、格納方法を修正するまで拡張を止める。Secret payloadやState本文を取得してこの確認を行わない。

## 生成・検証

[plan_read_access.py](../scripts/plan_read_access.py)はCore/MyPage共通の定義と差分生成器である。既定の実行はmanifestの検証だけでクラウドに接続しない。

```bash
python3 infra/gcp/scripts/plan_read_access.py
```

固定CIの `terraform-static` は、trusted controllerからイメージへコピーした [plan_policy.py](../../../.github/ci-validation/plan_policy.py) で、対象commitのJSONだけを検証する。対象commitのPythonを固定検査で実行しない。manifestがない過去commitも固定検査で停止するため、手動実行の対象SHAはmanifestを含むものを選ぶ。以前のHCLを使う場合も、レビュー済みのmanifestを含むソースとして検査し直す。テストは別コンテナで実行し、manifest・生成器・テストの変更はCore/MyPage両方のCIへ分類する。

Coreの `bootstrap_normal_ops.py` はPlanをこのmanifestから生成し、Applyは従来のcatalog/ROOTSから独立して生成する。Planの新familyをApply生成へ流用しない。既存共有roleの本文は広げず、旧Plan bindingから新roleへの移行には次の専用ツールを使う。通常bootstrapは移行済みの新role/bindingを読み取り確認するまでwrite処理へ進まない。MyPage Applyが共有している旧read roleも変更しない。

## 読み取り棚卸しとdry-run

管理端末の既存gcloud認証を使う。Plan SAへ棚卸し用権限を追加したり、Plan SAをimpersonateして試行しない。必要なIAM/Cloud Assetの読み取り権限やAPIが足りない場合は停止し、通常bootstrapを代用しない。

```bash
# 保存先は既存のprivateなディレクトリ。新しいファイルを指定する。
python3 infra/gcp/scripts/plan_read_access.py --live --export /private/plan-before.json
# 取得済みsnapshotによるオフラインの差分確認（クラウド接続・反映不可）
python3 infra/gcp/scripts/plan_read_access.py --snapshot /private/plan-before.json
```

project/SAの同一性・有効状態、user-managed keyなし、project policy etag、旧role/new roleの期待範囲、permissionのGA/custom-role対応を確認する。無効な未使用APIは自動有効化しない。Cloud Assetはidentityを指定しresource/accessのfilterなしで解析し、`fullyExplored`、エラーなし、project policyとの一致を要求する。条件付き・間接・子resourceの追加grant、想定外role、既存禁止permission、解析不足を検出した場合は停止する。既存grantを勝手に削除して続行しない。

出力はpermissionの追加/削除、固定されたPlan対象名、確認digest、review待ちの状態に限定し、raw IAM policyを公開ログへ出さない。exportは0600で新規作成し、既存ファイルを上書きしない。snapshot、復旧記録、review記録はGit・public Issue・artifactへ保存しない。

## 反映前のレビュー

project scopeのAnalyzerだけでは、対象外resource type、他projectへのgrant、service account経由の間接アクセス、Kubernetes RBAC等の別認可、設定内の平文秘密値を網羅したとは扱えない。管理者は残る範囲を別途確認し、privateな根拠記録を作る。旧PR専用identityが残っている場合も、現行の権限拡張対象へ加えず、workflow/Environment/trust/利用状況と整理方針を記録する。存在確認と利用可能性の確認は区別する。

review JSONは次のfieldを持たせる。すべての確認が終了した場合だけ対応するbooleanをtrueにする。ツールはこの記録を管理者の確認として扱い、技術的な完全証明として扱わない。

| field | 内容 |
| --- | --- |
| `confirmation` | 最新dry-runのdigest。manifestと取得した全snapshotを束縛する |
| `reviewed_at` | timezone付きISO日時。未来日時と24時間超を拒否 |
| `evidence_reference` | privateな確認根拠の参照先 |
| `resource_iam_review_complete` | Analyzer対象外を含むresource IAM確認済み |
| `cross_project_grants_review_complete` | 他project・継承・間接経路確認済み |
| `plaintext_secrets_absent` | 許可APIで読める設定に平文秘密値がないことを確認済み |
| `no_other_members_of_plan_role` | 新common roleが子resourceを含め他identityへ付与されていないことを確認済み |
| `legacy_paths_reviewed` | 旧経路の残存理由・利用状況と整理方針を確認済み |

CLIの `--live` 成功やコードのマージだけでは環境反映可能とは判断しない。review記録がないdry-runは `blocked_pending_operator_review: true` を出力する。

## 明示的な環境反映指示を受けた場合

コード・CI・棚卸しを完了し、実行中のDEV Planを止め、復旧用のprivate保存先を用意する。既存State/lock・AWS trust・PROD・Apply SA・共有role本文は変更しない。PR再実行用WIF変更は [workflow設計](../../../docs/development/ci-cd-workflows.md#45-pr-plan-の再実行を有効にするowner) の別手順で行う。

```bash
# 同じmanifestとfresh IAMに対するreviewを管理者が作った後にだけ実行する。
# digestは実際のdry-run出力を指定。backupは存在しないディレクトリを指定する。
python3 infra/gcp/scripts/plan_read_access.py --live --apply \
  --confirm ACTUAL_DRY_RUN_DIGEST --review /private/review.json \
  --backup /private/plan-access-backup
```

ツールは再取得したIAMとreviewのdigestを照合し、0700のbackupディレクトリに更新前snapshotと提案内容を0600で保存してから、新common roleだけを作成/更新する。role本文のreadback後、project policyをetag付きで更新し、2 Plan SAの旧bindingだけを新roleへ置き換える。共有roleの他member、Apply、audit configを含むその他のpolicyは保存し、readbackで比較する。既存common roleの更新にもetagを用いる。

permissionの非対応、解析不足、review不一致、etag競合、readback不一致は停止条件である。途中失敗で自動的に次の段階へ進まない。特にrole作成成功・project policy更新失敗時はroleだけ残るため、実状態を読み戻して復旧か再dry-runを選ぶ。

反映後はresource IAMも再確認し、2 Plan SAから旧project bindingだけが外れたこと、Apply/PROD/AWSと共有role本文が不変であることを照合する。その後、指示された範囲でCore/MyPageのnative Planと再実行を確認し、実行結果をIssue/PRに記録する。Secret実値・State本文・raw plan JSONは公開しない。

## 復旧

1. 実行中のDEV Planを停止する。復旧自体も環境更新なので明示的な指示の範囲で行う。
2. backupの `before.json` と最新のproject policyをprivateに取得する。**古いproject policy全体をそのまま上書きしない。** 最新policyのetagと他member・audit configを維持し、対象2 Plan SAのbindingだけをbackupと同じrole/conditionへ戻したpolicyを作る。
3. `gcloud projects set-iam-policy test-youtube-study-space /private/restored-policy.json` でetag付き更新を行い、最新値を取得して対象以外のbindingが変わらないことを照合する。競合したら再取得して作り直す。
4. common roleが更新前から存在していた場合、他memberへの新たな付与がないことを確認し、現在etagを含めて保存済みrole定義へ戻しreadbackする。新規作成した場合はbinding解除だけでよく、role削除を復旧の必須条件にしない。共有legacy role、旧SA/WIF、Stateには触れない。
5. 新token発行の停止と、発行済みtokenの有効期間を考慮した権限確認を別々に行う。binding解除が既存tokenを即時失効させるとは説明しない。

棚卸し結果、未確認事項、個々の実行記録は [Issue #1562](https://github.com/kani3camp/youtube-study-space/issues/1562) で管理する。
