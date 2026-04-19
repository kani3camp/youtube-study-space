## AWS SSO（認証）

プロファイルが IAM Identity Center（旧 AWS SSO）経由（`~/.aws/config` に `sso_session` などがある）のときは、**トークンの期限切れで CDK や AWS CLI が失敗する**。その場合は再ログインする。

```bash
aws sso login --profile プロファイル名
```

動作確認:

```bash
aws sts get-caller-identity --profile プロファイル名
```

## Useful commands

それぞれ`--profile プロファイル名`を付加する。（場合によっては region 指定も）

- `pnpm build` compile typescript to js
- `pnpm watch` watch for changes and compile
- `pnpm test` perform the jest unit tests
- `pnpm cdk:bootstrap` 当該 AWS アカウント環境で初めての場合
- `pnpm cdk:deploy` deploy this stack to your default AWS account/region
- `pnpm cdk:diff` compare deployed stack with current state
- `pnpm cdk:synth` emits the synthesized CloudFormation template

## 日次バッチと通知の運用メモ

- 日次バッチ（Fargate）は Step Functions で直列実行（00:00:15 JST開始）。
- 失敗通知は SNS Topic 経由で `sns_notify_discord` Lambda が Discord へ送信。
- Lambdaの Errors>0 と Step Functions ExecutionsFailed>0 のアラームをSNSに連携。
- 主要出力（CfnOutput）:
  - `BatchClusterArn`, `DailyBatchTaskDefinitionArn`, `BatchSecurityGroupId`, `BatchPublicSubnetIds`, `BatchVpcId`, `DailyBatchStateMachineArn`

