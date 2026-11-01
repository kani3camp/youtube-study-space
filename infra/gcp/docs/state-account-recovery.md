# State account recovery / contacts (#1165)

これはTerraform専用AWS member accountの運用runbookです。contact値、account ID、
SSO URL、MFA device情報、回復コード、請求情報はprivate正本で扱い、公開Issueへ転記しません。
State importやproduction基盤構築とは独立したworkstreamです。

## 現在の確認範囲

2026-10-14に[Issue #1165](https://github.com/kani3camp/youtube-study-space/issues/1165)
本文と最新commentを再確認しました。contact正本とrecovery/payment責任者が未確定という
公開記録があります。今回のcheckoutからprivate正本と認証済みAWS accountを確認できて
いないため、contact未設定・MFA未設定・root credentialなしを現在の実測結果とは扱いません。
既存Monitoring通知先やGitHubの公開profileからcontactを推定しません。

## Private checklist

各行は値そのものを公開せず、確認日時、確認主体、private正本への参照、PASS/STOPだけを
既存Issueへ記録します。同じ人を複数区分へ割り当てる場合も明示的な責任確認が必要です。

| 項目 | Privateで確認するもの | 完了証拠 |
| --- | --- | --- |
| BILLING | 請求連絡担当、到達可能な連絡先、payment管理権限 | account `GetAlternateContact`のfresh read-backと到達確認 |
| OPERATIONS | State障害担当、代替担当、連絡手段 | fresh read-backと担当の確認 |
| SECURITY | trust/credential incident担当、代替担当 | fresh read-backと担当の確認 |
| MFA | Identity Center/外部IdPの実際の認証policy、primary/secondary authenticator | 通常operatorのMFAを要求するlogin実測 |
| Break-glass | SSO/IdP障害時の独立経路、発動条件、承認主体、期限 | 既存経路のpermission/read-only到達検証 |
| AWS Organizations | 管理accountまたはdelegated administratorの復旧能力 | 実際の管理経路と許可されたtask確認 |
| Root | member accountのroot credential現況、centralized root accessの有無 | 認証済み監査。必要のないroot credentialを発行しない |
| 責任 | recovery実行担当、payment責任者、代替者 | 本人がprivate正本で確定 |

Identity CenterのMFAはIAM userの`aws:MultiFactorAuthPresent`条件だけでは証明しません。
既存SSO/IdPの設定と実loginで確認します。Break-glass用の新規IAM user、長期access key、
root credentialをこのrunbookのために発行しません。centralized root accessを既に持つ
組織では、利用可能な`AssumeRoot` taskと担当を確認し、full root sessionを仮定しません。

## Incident procedure

1. apply/importを停止し、既存GitHub gatesを閉じる。Required reviewers/trustを迂回しない。
2. Private正本から担当へ連絡し、対象account、environment、exact State keyを照合する。
3. 通常SSOが利用できれば短期operator sessionを使用する。利用不能なら確認済みの
   break-glass経路を担当が発動する。復旧経路が不明ならcloud mutationをSTOPする。
4. `GetCallerIdentity`、対象bucket metadata/versioning、exact keyのcurrent versionと
   native lockをread-onlyで確認する。403/通信失敗を「存在しない」と解釈しない。
5. `ListObjectVersions`/exact version GETで事故前後をprivateに比較する。現行Stateと
   後続正常write、lineage/serial、実resourceの一致を確認し、選択理由を残す。
6. Lock所有者・runを確認せずforce-unlockしない。古いversionのblind restore、dev/prod間
   copy、State履歴やlock versionの削除を行わない。
7. 復旧writeが必要なら、exact対象・最新State保全・native lock・rollback/費用条件を
   確定した個別packetで実行する。このrunbookだけをrestore実行権限と解釈しない。
8. 復旧後にsame-source protected full-root plan、実resource整合、lock解放を検証する。
   Unknown/driftが説明できるまでapplyしない。担当がincident closureを記録する。

復旧訓練は既存認証経路からのread-only到達確認を先に行います。SSOを故意に無効化したり、
本番Stateを壊したりして訓練しません。restore訓練に新object/保存費が必要なら別途範囲を確認します。

## 残る本人判断

Private正本を読んだ後も未確定の場合に限り、各contact・recovery/payment責任者・代替者を
本人に確定してもらいます。値が取得できる場合は再入力を求めません。Issueは設定/read-back、
MFA/login、break-glass到達、責任確認が揃うまでOPENを維持します。

References: [alternate contacts](https://docs.aws.amazon.com/accounts/latest/reference/manage-acct-update-contact-alternate.html),
[centralized root access](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_root-user.html)。
