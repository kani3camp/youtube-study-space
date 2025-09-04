## Useful commands

それぞれ`--profile プロファイル名`を付加する。（場合によっては region 指定も）

- `npm run build` compile typescript to js
- `npm run watch` watch for changes and compile
- `npm run test` perform the jest unit tests
- `cdk bootstrap` 当該 AWS アカウント環境で初めての場合
- `npx cdk deploy` deploy this stack to your default AWS account/region
- `npx cdk diff` compare deployed stack with current state
- `npx cdk synth` emits the synthesized CloudFormation template

## 日次バッチと通知の運用メモ

- 日次バッチ（Fargate）は Step Functions で直列実行（00:00:15 JST開始）。
- 失敗通知は SNS Topic 経由で `sns_notify_discord` Lambda が Discord へ送信。
- Lambdaの Errors>0 と Step Functions ExecutionsFailed>0 のアラームをSNSに連携。
- 主要出力（CfnOutput）:
  - `BatchClusterArn`, `DailyBatchTaskDefinitionArn`, `BatchSecurityGroupId`, `BatchPublicSubnetIds`, `BatchVpcId`, `DailyBatchStateMachineArn`
