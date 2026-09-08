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

- 日次バッチ: EventBridge Scheduler が **00:00 JST** に `start_daily_batch` を実行 → Step Functions 起動。**SFN は先頭で 15 秒 Wait** したうえで ECS Fargate を直列実行（`reset-daily-total` → `update-rp` → `transfer-bq`）。
- 失敗通知は SNS Topic 経由で `sns_notify_discord` Lambda が Discord へ送信。
- Lambdaの Errors>0 と Step Functions ExecutionsFailed>0 のアラームをSNSに連携。
- 主要出力（CfnOutput）:
  - `BatchClusterArn`, `DailyBatchTaskDefinitionArn`, `BatchSecurityGroupId`, `BatchPublicSubnetIds`, `BatchVpcId`, `DailyBatchStateMachineArn`
- `AlarmEmail` パラメータにメールアドレスを指定すると、`AlarmsTopic` に Email subscription を追加する。未指定の場合は Email subscription を作らない。
  - メール通知も有効にして deploy する例:
    ```bash
    pnpm cdk:deploy --profile プロファイル名 --parameters AlarmEmail=notify@example.com
    ```
  - 初回のみ、指定したメールに AWS から届く **Confirm subscription** のリンクを開いて承認する。コンソールから行う場合は **SNS → Topics → `AlarmsTopic` に相当するトピック → Subscriptions** で Pending を Confirm する。


## youtube-bot Fargate Service

The stack defines a dedicated `youtube-bot` Fargate Service on the existing batch VPC/cluster.

Safety defaults:

- `YoutubeBotDesiredCount=0`: deploying the stack alone does not start a bot task.
- `YoutubeBotEnvironment=disabled`: the WIF/headless container rejects startup until an explicit `development` or `production` target is deployed.
- deployment uses minimum healthy percent 0 / maximum healthy percent 100, so desired count 1 replaces the old task before starting the new task instead of temporarily running two bot tasks.
- unexpected task exits (`EssentialContainerExited` / `TaskFailedToStart`) are forwarded to `AlarmsTopic`.
- the bot has a dedicated task role, log group, and security group. Public-IP HTTPS egress is used to avoid adding a NAT gateway.

New outputs:

- `YoutubeBotTaskDefinitionArn`
- `YoutubeBotTaskRoleArn`
- `YoutubeBotServiceName`
- `YoutubeBotSecurityGroupId`

The safe rollout order is:

1. deploy the environment with `YoutubeBotDesiredCount=0` and the correct `YoutubeBotEnvironment`;
2. take `YoutubeBotTaskRoleArn` from the stack output and add that role to the environment's Google WIF provider condition and Service Account `workloadIdentityUser` binding;
3. run the task definition once with container command `preflight`;
4. only after preflight succeeds, stop the streaming-PC bot;
5. set `YoutubeBotDesiredCount=1`;
6. verify live-chat command/moderation behavior.

Do not set desired count 1 before the old streaming-PC bot is stopped.
