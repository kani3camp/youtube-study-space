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

## Google Cloud WIF / CDK deploy

AWS上のLambda/FargateからGoogle CloudへアクセスするワークロードはWorkload Identity Federationを使用する。WIF用CloudFormationパラメータは必須で、空文字はデプロイ前に拒否される。

このリポジトリは Online Study Space の実システム用なので、秘密ではない環境識別子は正本として管理する。AWS access key / secret access key / session token、Google service account private key / JSON key、OAuth client secret、API token、passwordなどの秘密値はコミットしない。

| Env | AWS profile | AWS account | GCP project | GCP project number | Google service account |
| --- | --- | --- | --- | --- | --- |
| dev | `soraride-dev` | `657533259235` | `test-youtube-study-space` | `48101442817` | `test-youtube-study-space@appspot.gserviceaccount.com` |
| prod | `soraride-prod` | `652333062396` | `youtube-study-space` | `906336399194` | `youtube-study-space@appspot.gserviceaccount.com` |

両環境ともWorkload Identity Poolは `aws-runtime`、Providerは `aws-provider`、CloudFormation stackは `AwsCdkStack`。`scripts/cdk-env.sh` が環境ごとのWIF audienceを組み立て、`aws sts get-caller-identity` でAWS account IDを照合してからCDKを実行する。

### dev

```bash
aws sso login --profile soraride-dev
bash scripts/cdk-env.sh dev diff
# diffを確認してから
bash scripts/cdk-env.sh dev deploy
```

### prod

```bash
aws sso login --profile soraride-prod
bash scripts/cdk-env.sh prod diff
# diffを確認してから
bash scripts/cdk-env.sh prod deploy
```

`deploy` はaccount照合後に `--require-approval never` で実行する。WIFパラメータを手入力する必要はない。

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
