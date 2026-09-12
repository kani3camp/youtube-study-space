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

このリポジトリは Online Study Space の実システムを運用するためのリポジトリであるため、秘密ではない環境識別子は運用事故を防ぐ目的で正本として記載する。一方、AWS access key / secret access key / session token、Google service account private key / JSON key、OAuth client secret、API token、passwordなどの認証情報・秘密値はリポジトリ、Issue、PR本文、ログへ記載しない。

### Production環境の識別子

- AWS profile: `soraride-prod`
- AWS account ID: `652333062396`
- CloudFormation stack: `AwsCdkStack`
- Google Cloud project ID: `youtube-study-space`
- Google Cloud project number: `906336399194`
- WIF audience: `//iam.googleapis.com/projects/906336399194/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider`
- Google service account: `youtube-study-space@appspot.gserviceaccount.com`

CloudFormationパラメータとの対応は以下。

- `GoogleCloudProject`: `youtube-study-space`
- `GcpWifAudience`: `//iam.googleapis.com/projects/906336399194/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider`
- `GcpWifServiceAccountEmail`: `youtube-study-space@appspot.gserviceaccount.com`

### Productionへのdiff / deploy

Productionではprofile名だけを信用せず、`aws sts get-caller-identity` で実際のAWS account IDを検証してからCDKを実行する。Stack名と各parameterの所属Stackも明示し、将来Stackが増えた場合の誤適用を避ける。

まずAWS SSOへログインする。

```bash
aws sso login --profile soraride-prod
```

次に、対象AWSアカウントを検証した上で `cdk:diff` を確認する。

```bash
(
  set -euo pipefail

  EXPECTED_AWS_ACCOUNT="652333062396"

  ACTUAL_AWS_ACCOUNT=$(aws sts get-caller-identity \
    --profile soraride-prod \
    --query Account \
    --output text)

  test "$ACTUAL_AWS_ACCOUNT" = "$EXPECTED_AWS_ACCOUNT"

  corepack pnpm cdk:diff AwsCdkStack \
    --profile soraride-prod \
    --parameters 'AwsCdkStack:GoogleCloudProject=youtube-study-space' \
    --parameters 'AwsCdkStack:GcpWifAudience=//iam.googleapis.com/projects/906336399194/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider' \
    --parameters 'AwsCdkStack:GcpWifServiceAccountEmail=youtube-study-space@appspot.gserviceaccount.com'
)
```

`cdk:diff` の内容を確認した後、同じAWSアカウント検証を通してdeployする。

```bash
(
  set -euo pipefail

  EXPECTED_AWS_ACCOUNT="652333062396"

  ACTUAL_AWS_ACCOUNT=$(aws sts get-caller-identity \
    --profile soraride-prod \
    --query Account \
    --output text)

  test "$ACTUAL_AWS_ACCOUNT" = "$EXPECTED_AWS_ACCOUNT"

  corepack pnpm cdk:deploy AwsCdkStack \
    --profile soraride-prod \
    --require-approval never \
    --parameters 'AwsCdkStack:GoogleCloudProject=youtube-study-space' \
    --parameters 'AwsCdkStack:GcpWifAudience=//iam.googleapis.com/projects/906336399194/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider' \
    --parameters 'AwsCdkStack:GcpWifServiceAccountEmail=youtube-study-space@appspot.gserviceaccount.com'
)
```

特に旧DynamoDBサービスアカウントJSON認証からWIFへ切り替える最初の更新では、3値を指定せずに進めないこと。CloudFormation Rulesは空値による実行時障害を防ぐための最後の防波堤であり、対象AWSアカウントの照合や `cdk:diff` の確認の代替ではない。

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
