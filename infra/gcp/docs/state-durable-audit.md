# State durable audit proposal (#1166)

これはsource/設定案です。trail/bucket/policyを作成せず、logging、継続課金、retentionを
有効化しません。[Issue #1166](https://github.com/kani3camp/youtube-study-space/issues/1166)
の2026-10-06設計を具体化します。既存org trail、管理account側設定、現在のmember accountの
trail有無は認証済みfresh inventoryが必要で、過去bootstrapのtrail0を現況とみなしません。

## 推奨する最小構成

通常CloudTrail trailを1本、Stateとは別のdedicated audit S3 bucketへ配送する案です。
既存organization trailで同じ要件が満たせる場合は再利用を先に評価し、重複copyを作りません。
新trailの場合はstate control-plane account、既存Stateと同じregion、multi-region、global
service events、log file validationを候補にします。CloudWatch Logs、Insights、network
activity events、data-event aggregation、定常Athena queryは初期構成に含めません。

対象はmanagement eventsのread/write、および**exact State bucketのS3 object data events**です。
Bucket全体をobject selectorにすることでdev/prod key、native lock、workspaceを含めます。
別のaudit bucketをselectorへ含めず、workload accountの全bucketを収集しません。
APIのevent記録対応を確認し、State/lockのGetObject/PutObject/DeleteObjectを実ログで検証します。
List/HEAD等の全APIが同じobject data eventとして記録されることは仮定しません。

[Advanced event selector source](../proposals/state-audit-selectors.example.json)の
`REPLACE_WITH_PRIVATE_STATE_BUCKET`をprivate inventoryのexact bucket名へ置換します。
`resources.ARN`は末尾`/`の`StartsWith`で、wildcard `*`ではありません。
`readOnly`と`eventName`を制限しないため、read/write両方を対象にします。
これは新trail向け候補です。`PutEventSelectors`はselector全体の置換なので、既存trailへ
そのまま適用しません。既存設定のread-backと差分確認が必要です。

## 保存先 / access boundary

| 項目 | 最小案 | 実装前の確認 |
| --- | --- | --- |
| 保存先 | Stateと別のaudit bucket、Block Public Access全面有効、TLS | bucket ownership、region、既存org destination |
| Encryption | SSE-S3 | 既存encryption契約。SSE-KMSが必要ならkey policy/追加費用 |
| CloudTrail write | service principal `cloudtrail.amazonaws.com` | exact trail ARNで`aws:SourceArn`を限定 |
| Bucket ACL check | `s3:GetBucketAcl`をexact audit bucketへ | CloudTrail serviceにだけ付与 |
| Log/digest write | `s3:PutObject`をexact `AWSLogs/<account-id>/*`へ | `s3:x-amz-acl = bucket-owner-full-control`とexact source ARN |
| Auditor | audit prefixのList/Get、必要なdigest validation | State read/writeやaudit Put/Deleteを追加しない |
| Terraform CI | audit bucketへの追加権限なし | State roleがaudit logsを変更できないnegative test |
| Tamper | log file validation | 改ざん検知であり削除防止ではない。Object Lock等は別判断 |

Source ARNにaccountを含むexact trail ARNを使用します。組織trailなら`AWSLogs/<org-id>/`
等のdestination契約が異なるためsingle-account policyをコピーしません。Private account ID、
bucket名、log中のidentity/source IP/object keyを公開Issue、artifact、Notionへ転記しません。

## Retention / cost comparison

| 案 | 利点 | 留意点 |
| --- | --- | --- |
| Event Historyのみ | 追加trail不要 | 90日managementのみ。object data events/永続監査を満たさない |
| S3 Standard 90日 | 保存量が小さい | 90日を超えるincident調査不可 |
| S3 Standard 365日 | 1年の監査、restore charge不要、最小構成候補 | lifecycle expirationと費用上限の本人判断が必要 |
| 長期archive transition | 長期保存量が大きい場合の候補 | 小さいlogのminimum storage/transition requests、restore費用/時間 |
| Object Lock | immutable retentionの候補 | 削除不可期間とincident運用が変わるため別判断 |

初期候補はStandard 365日expiration、transition無しです。これは確定retentionでも承認済み
課金でもありません。Bucket versioningを採用する場合はnoncurrent version/delete marker
retentionも決め、current-object expirationだけで保管費が止まるとみなしません。

2026-10-14に[AWS公式CloudTrail料金](https://aws.amazon.com/cloudtrail/pricing/)を確認しました。
S3配送のfirst management copyは無料、追加management copyは$2.00/100,000、
data eventsは$0.10/100,000です。例は月1,000 data eventsで$0.001、100,000で$0.10、
1,000,000で$1.00（CloudTrail data-event部分のみ）です。
既存organization trailとの重複を確認するまでfirst-copy無料を予算に確定しません。

月額見積もりは`data_events × 0.10 / 100000 + extra_management_events × 2 / 100000`
に、選択regionのS3 storage/PUT/GET/LIST、version保持、必要時のKMS/query/restoreを加えます。
現在のevents数、圧縮log量、S3 request数、累積費用は未実測であり、総額はunknownです。
新たな継続課金を有効化する前にretention、上限、通知/責任者を確定します。

## 実装 / acceptance packet

1. Authenticated inventory: identity、全regionのtrail（shadow/org含む）、status、event selectors、
   destinations、bucket policy/encryption/versioning/lifecycle、既存auditor境界をprivate確認する。
2. 重複・費用・retentionを確定し、exact source diffとrollbackをレビューする。STOP時は
   設定案だけ保持する。記録を失わせるtrail/log削除をrollbackにしない。
3. 承認された場合にだけloggingを有効化する。delivery status、fresh log/digest配送を確認する。
4. 既存の承認済みprotected planの自然なState/lock read/writeを使ってactor/event/keyを照合する。
   監査のためにproduction State内容を変更しない。
5. Unauthorized identityのaudit read/write、State CIのaudit writeが拒否されることを検証する。
   拒否されるべきAPIがloggingされる範囲も確認し、未記録を「試行なし」と解釈しない。
6. Digest validation、log到達/retention、費用計測方法、担当を確認して既存Issueへsanitized
   PASS/STOPを記録する。CloudTrailは開始前のS3 data eventsを遡及復元しません。

References: [advanced field selectors](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_AdvancedFieldSelector.html),
[CloudTrail S3 bucket policy](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/create-s3-bucket-policy-for-cloudtrail.html),
[S3 pricing](https://aws.amazon.com/s3/pricing/)。
