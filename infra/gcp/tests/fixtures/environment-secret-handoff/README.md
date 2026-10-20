# Environment secret handoff: inactive synthetic repro

このfixtureは `.github/workflows` の外にあり、GitHub上で登録・実行されません。
fixture自体はsource gate、既存workflow、IAM、Environment、secretを変更しません。
ローカル検証はshellの分類・非公開出力を確認するだけで、GitHubのsecret配送やsnapshotを再現しません。

## Confirmed contract and remaining questions

auditのidentity入力が空なら、既存mask stepは認証前に停止します。
空の原因は、保存先・保存内容・GitHubの配送それぞれの証拠を分けて調べます。
`secrets: inherit` の追加やrepo-wide secret化を、未確定原因の修復として先行しません。

別のsource候補は、既存Terraform workflowと同じoptional secret宣言/caller mappingを
audit専用2名だけに追加します。gateはfalseのままです。この候補でGitHub上の配送が直ることは未検証です。
repository内の先例は[optional宣言 #1170](https://github.com/kani3camp/youtube-study-space/pull/1170)と
[明示caller mapping #1171](https://github.com/kani3camp/youtube-study-space/pull/1171)です。
#1171の記録では宣言だけで空値が解消しなかったため、両側の名前契約を追加しています。
この先例と現sourceの欠落は修正候補の根拠であり、今回の配送失敗のlive根因証明とは区別します。

[公式reusable workflow仕様](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
は、called job自身のEnvironment secretsを使用することを説明しています。
[公式取得タイミング](https://docs.github.com/en/actions/reference/security/secrets)
ではrepository/organization secretsはrun queue時、Environment secretsは参照job開始時に取得されます。
したがって、Environment secretをdispatch後に追加したという事実だけでは空値の原因を確定できません。

保存receiptはprivateで確認します。必要な情報は、実PUT endpointのrepository/Environment/secret名、
成功status・時刻、LIST metadata、暗号化前入力が非空で所定形式だったかというbooleanです。
secretの値・暗号文・token・private identity全文を公開しません。LIST成功だけでは値の非空性や配送を証明できません。

## Local verification

repository rootから以下を実行します。API call、GitHub run、secret設定はありません。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 .github/scripts/test_gcp_user_activity_schema_audit_workflow.py
```

既存mask stepの両方空・片方空・不正形式を合成入力で検証します。
fixtureのprobeは、未設定・正常な合成値・想定外値について `absent / expected / unexpected` だけを出します。
stdoutへ値を出さず、summaryにも値・length・hashを出しません。

## Future GitHub comparison requires separate approval

この手順は未実行です。GitHub runやEnvironment/secret追加の承認を別途得た場合にだけ、
既存cloud accessから隔離されたtest repositoryで合成Environmentを使います。
合成secret名は `SYNTHETIC_A` / `SYNTHETIC_B`、値は公開dummy `synthetic-alpha` / `synthetic-beta` です。
実identityやcredentialを投入しません。fixtureにはOIDC、checkout、cloud action、query、artifact uploadがありません。

承認済みのtest repositoryへcallerを `.github/workflows/synthetic-secret-handoff-caller.yml`、
reusableを `.github/workflows/synthetic-secret-handoff-reusable.yml`、
declaredを `.github/workflows/synthetic-secret-handoff-declared.yml` として配置します。
同じ `synthetic-secret-handoff` Environmentを参照するdirect job、宣言なしreusable、
optional2名宣言/caller mapping付きreusableを比較し、
workflow SHA、run/attempt、job開始・approval・secret metadataの時刻と分類だけをprivate記録します。

| Separate approved experiment | Preparation | Interpretation |
| --- | --- | --- |
| Baseline | dummy secretsをdispatch前に設定 | 各jobがexpectedになるか確認 |
| Timing | 同じ保護設定でjobを待機させ、dummy secretsを設定してから承認 | baselineとの差を確認。job開始/snapshot境界は結果だけから断定しない |

各experimentは別run承認です。失敗を見て自動retryしません。
directと宣言付きreusableがexpected、宣言なしだけabsentなら、explicit contract差分の証拠になります。
directだけexpectedなら、reusable配送差分を引き続き調べます。
全部absentなら、保存先・値・Environment選択・snapshotを引き続き調べます。
全部expectedでも、過去のaudit runへ正しく配送されたことは証明しません。
実験後のfixture/合成secret/Environment撤去も、その実験の承認scopeで行います。
