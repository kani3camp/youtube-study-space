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

## Google Cloud WIF パラメータ

AWS上のLambda/FargateからGoogle CloudへアクセスするワークロードはWorkload Identity Federationを使用する。以下のCloudFormationパラメータはすべて必須で、空文字の場合はCloudFormation Rulesによりリソース更新前にデプロイを拒否する。

- `GoogleCloudProject`: Google Cloud project ID
- `GcpWifAudience`: Workload Identity Providerの完全なaudience（`//iam.googleapis.com/projects/.../providers/...`）
- `GcpWifServiceAccountEmail`: WIF経由でimpersonationするGoogle service accountのメールアドレス

WIF移行を含むスタック更新では、対象環境の値を確認して `cdk:diff` と `cdk:deploy` の両方へ明示する。実値をリポジトリ、Issue、PR本文、ログへ貼り付けないこと。

```bash
pnpm cdk:diff --profile プロファイル名 \
  --parameters GoogleCloudProject='<project-id>' \
  --parameters GcpWifAudience='<provider-audience>' \
  --parameters GcpWifServiceAccountEmail='<service-account-email>'

pnpm cdk:deploy --profile プロファイル名 \
  --parameters GoogleCloudProject='<project-id>' \
  --parameters GcpWifAudience='<provider-audience>' \
  --parameters GcpWifServiceAccountEmail='<service-account-email>'
```

特に旧DynamoDBサービスアカウントJSON認証からWIFへ切り替える最初の更新では、3値を指定せずに進めないこと。CloudFormation Rulesは空値による実行時障害を防ぐための最後の防波堤であり、適切な対象環境の値を確認する手順の代替ではない。

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
