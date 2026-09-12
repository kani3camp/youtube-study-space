## AWS SSO（認証）

プロファイルが IAM Identity Center（旧 AWS SSO）経由（`~/.aws/config` に `sso_session` などがある）のときは、**トークンの期限切れで CDK や AWS CLI が失敗する**。その場合は再ログインする。

```text
aws sso login --profile プロファイル名
```

動作確認:

```text
aws sts get-caller-identity --profile プロファイル名
```

## Useful commands

それぞれ`--profile プロファイル名`を付加する。（場合によっては region 指定も）

- `pnpm build` compile typescript to js
- `pnpm watch` watch and compile
- `pnpm test` perform the jest unit tests
- `pnpm cdk:bootstrap` 当該 AWS アカウント環境で初めての場合
- `pnpm cdk:deploy` deploy this stack to your default AWS account/region
- `pnpm cdk:diff` compare deployed stack with current state
- `pnpm cdk:synth` emits the synthesized CloudFormation template

## Google Cloud WIF / CDK deploy

AWS上のLambda/FargateからGoogle CloudへアクセスするワークロードはWorkload Identity Federationを使用する。WIF用CloudFormationパラメータは必須で、空文字はデプロイ前に拒否される。

このリポジトリは Online Study Space の実システム用なので、秘密ではない環境識別子は正本として管理する。AWS access key / secret access key / session token、Google service account private key / JSON key、OAuth client secret、API token、passwordなどの秘密値はコミットしない。

| Env | AWS profile | AWS account | GCP project | GCP project number | Google service account |
| --- | --- | --- | --- | --- | --- |
| dev | `soraride-dev` | `657533259235` | `test-youtube-study-space` | `48101442817` | `test-youtube-study-space@appspot.gserviceaccount.com` |
| prod | `soraride-prod` | `652333062396` | `youtube-study-space` | `906336399194` | `youtube-study-space@appspot.gserviceaccount.com` |

両環境ともWorkload Identity Poolは `aws-runtime`、Providerは `aws-provider`、CloudFormation stackは `AwsCdkStack`。

以下は **macOS の zsh/bash と Windows PowerShell で同じまま実行できるよう、シェル固有の改行継続や変数を使わない1行コマンド**にしている。リポジトリルートから実行する場合は最初に `cd aws-cdk` する。

### dev

```text
cd aws-cdk
aws sso login --profile soraride-dev
pnpm cdk:diff AwsCdkStack --profile soraride-dev --parameters "AwsCdkStack:GoogleCloudProject=test-youtube-study-space" --parameters "AwsCdkStack:GcpWifAudience=//iam.googleapis.com/projects/48101442817/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider" --parameters "AwsCdkStack:GcpWifServiceAccountEmail=test-youtube-study-space@appspot.gserviceaccount.com"
pnpm cdk:deploy AwsCdkStack --profile soraride-dev --require-approval never --parameters "AwsCdkStack:GoogleCloudProject=test-youtube-study-space" --parameters "AwsCdkStack:GcpWifAudience=//iam.googleapis.com/projects/48101442817/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider" --parameters "AwsCdkStack:GcpWifServiceAccountEmail=test-youtube-study-space@appspot.gserviceaccount.com"
```

`cdk:diff` の内容を確認してから `cdk:deploy` を実行する。

### prod

```text
cd aws-cdk
aws sso login --profile soraride-prod
pnpm cdk:diff AwsCdkStack --profile soraride-prod --parameters "AwsCdkStack:GoogleCloudProject=youtube-study-space" --parameters "AwsCdkStack:GcpWifAudience=//iam.googleapis.com/projects/906336399194/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider" --parameters "AwsCdkStack:GcpWifServiceAccountEmail=youtube-study-space@appspot.gserviceaccount.com"
pnpm cdk:deploy AwsCdkStack --profile soraride-prod --require-approval never --parameters "AwsCdkStack:GoogleCloudProject=youtube-study-space" --parameters "AwsCdkStack:GcpWifAudience=//iam.googleapis.com/projects/906336399194/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider" --parameters "AwsCdkStack:GcpWifServiceAccountEmail=youtube-study-space@appspot.gserviceaccount.com"
```

`cdk:diff` の内容を確認してから `cdk:deploy` を実行する。

## 日次バッチと通知の運用メモ

- 日次バッチ: EventBridge Scheduler が **00:00 JST** に `start_daily_batch` を実行 → Step Functions 起動。**SFN は先頭で 15 秒 Wait** したうえで ECS Fargate を直列実行（`reset-daily-total` → `update-rp` → `transfer-bq`）。
- 失敗通知は SNS Topic 経由で `sns_notify_discord` Lambda が Discord へ送信。
- Lambdaの Errors>0 と Step Functions ExecutionsFailed>0 のアラームをSNSに連携。
- 主要出力（CfnOutput）:
  - `BatchClusterArn`, `DailyBatchTaskDefinitionArn`, `BatchSecurityGroupId`, `BatchPublicSubnetIds`, `BatchVpcId`, `DailyBatchStateMachineArn`
- `AlarmEmail` パラメータにメールアドレスを指定すると、`AlarmsTopic` に Email subscription を追加する。未指定の場合は Email subscription を作らない。
  - メール通知も有効にして deploy する例:
    ```text
    pnpm cdk:deploy --profile プロファイル名 --parameters AlarmEmail=notify@example.com
    ```
  - 初回のみ、指定したメールに AWS から届く **Confirm subscription** のリンクを開いて承認する。コンソールから行う場合は **SNS → Topics → `AlarmsTopic` に相当するトピック → Subscriptions** で Pending を Confirm する。
