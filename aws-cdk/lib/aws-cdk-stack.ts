import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda'
import * as iam from 'aws-cdk-lib/aws-iam'
import { Platform } from "aws-cdk-lib/aws-ecr-assets";
import * as path from 'path';
import { fileURLToPath } from 'url';
import { aws_apigateway } from 'aws-cdk-lib';
import * as events from 'aws-cdk-lib/aws-events'
import * as targets from 'aws-cdk-lib/aws-events-targets'

export class AwsCdkStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);
    
    // カスタムポリシー
    const customPolicyDynamoDB = new iam.PolicyStatement({
      actions: ['dynamodb:GetItem'],
      effect: iam.Effect.ALLOW,
      resources: [
        'arn:aws:dynamodb:*:*:table/secrets'
      ]
    })
    
    // Lambda function
    const setDesiredMaxSeatsFunction = new lambda.DockerImageFunction(this, 'set_desired_max_seats', {
      functionName: 'set_desired_max_seats',
      code: lambda.DockerImageCode.fromImageAsset(path.join(__dirname, '../../system/'), {
        buildArgs: {
          HANDLER: 'main',
        },
        platform: Platform.LINUX_AMD64,
        entrypoint: ['/app/set_desired_max_seats'],
      }),
      timeout: cdk.Duration.seconds(20),
      reservedConcurrentExecutions: undefined,
    });
    (setDesiredMaxSeatsFunction.role as iam.Role).addToPolicy(customPolicyDynamoDB);
    
    const youtubeOrganizeDatabaseFunction = new lambda.DockerImageFunction(this, 'youtube_organize_database', {
      functionName: 'youtube_organize_database',
      code: lambda.DockerImageCode.fromImageAsset(path.join(__dirname, '../../system/'), {
        buildArgs: {
          HANDLER: 'main',
        },
        platform: Platform.LINUX_AMD64,
        entrypoint: ['/app/youtube_organize_database'],
      }),
      timeout: cdk.Duration.seconds(50),
      reservedConcurrentExecutions: 1,
    });
    (youtubeOrganizeDatabaseFunction.role as iam.Role).addToPolicy(customPolicyDynamoDB);

    const dailyOrganizeDatabaseFunction = new lambda.DockerImageFunction(this, 'daily_organize_database', {
      functionName: 'daily_organize_database',
      code: lambda.DockerImageCode.fromImageAsset(path.join(__dirname, '../../system/'), {
        buildArgs: {
          HANDLER: 'main',
        },
        platform: Platform.LINUX_AMD64,
        entrypoint: ['/app/daily_organize_database'],
      }),
      timeout: cdk.Duration.minutes(15),
      reservedConcurrentExecutions: 1,
    });
    (dailyOrganizeDatabaseFunction.role as iam.Role).addToPolicy(customPolicyDynamoDB);
    
    const checkLiveStreamStatusFunction = new lambda.DockerImageFunction(this, 'check_live_stream_status', {
      functionName: 'check_live_stream_status',
      code: lambda.DockerImageCode.fromImageAsset(path.join(__dirname, '../../system/'), {
        buildArgs: {
          HANDLER: 'main',
        },
        platform: Platform.LINUX_AMD64,
        entrypoint: ['/app/check_live_stream_status'],
      }),
      timeout: cdk.Duration.seconds(10),
      reservedConcurrentExecutions: undefined,
    });
    (checkLiveStreamStatusFunction.role as iam.Role).addToPolicy(customPolicyDynamoDB);
    
    const transferCollectionHistoryBigqueryFunction = new lambda.DockerImageFunction(this, 'transfer_collection_history_bigquery', {
      functionName: 'transfer_collection_history_bigquery',
      code: lambda.DockerImageCode.fromImageAsset(path.join(__dirname, '../../system/'), {
        buildArgs: {
          HANDLER: 'main',
        },
        platform: Platform.LINUX_AMD64,
        entrypoint: ['/app/transfer_collection_history_bigquery'],
      }),
      timeout: cdk.Duration.minutes(15),
      reservedConcurrentExecutions: 1,
    });
    (transferCollectionHistoryBigqueryFunction.role as iam.Role).addToPolicy(customPolicyDynamoDB);
    
    const processUserRPParallelFunction = new lambda.DockerImageFunction(this, 'process_user_rp_parallel', {
      functionName: 'process_user_rp_parallel',
      code: lambda.DockerImageCode.fromImageAsset(path.join(__dirname, '../../system/'), {
        buildArgs: {
          HANDLER: 'main',
        },
        platform: Platform.LINUX_AMD64,
        entrypoint: ['/app/process_user_rp_parallel'],
      }),
      timeout: cdk.Duration.minutes(15),
      reservedConcurrentExecutions: undefined,
    });
    (processUserRPParallelFunction.role as iam.Role).addToPolicy(customPolicyDynamoDB);
    
    
    // API Gateway
    const restApi = new aws_apigateway.RestApi(this, 'youtube-study-space-rest-api', {
      deployOptions: {
        stageName: 'default'
      },
      restApiName: 'youtube-study-space-rest-api',
      defaultMethodOptions: { apiKeyRequired: true },
      defaultCorsPreflightOptions: {
        allowOrigins: aws_apigateway.Cors.ALL_ORIGINS,
        allowMethods: aws_apigateway.Cors.ALL_METHODS,
        allowHeaders: aws_apigateway.Cors.DEFAULT_HEADERS,
        statusCode: 200,
      }
    });
    
    const apiKey = restApi.addApiKey('youtube-study-space-api-key', { apiKeyName: `youtube-study-space-api-key` });
    const plan = restApi.addUsagePlan('UsagePlan', { name: `youtube-study-space` });
    plan.addApiKey(apiKey);
    plan.addApiStage({ stage: restApi.deploymentStage });
    
    const apiSetDesiredMaxSeats = restApi.root.addResource('set_desired_max_seats')
    apiSetDesiredMaxSeats.addMethod(
      "POST", 
      new aws_apigateway.LambdaIntegration(setDesiredMaxSeatsFunction)
    )
    
    // EventBridge
    new events.Rule(this, '1minute', {
      schedule: events.Schedule.rate(cdk.Duration.minutes(1)),
      targets: [new targets.LambdaFunction(youtubeOrganizeDatabaseFunction)]
    })
    new events.Rule(this, 'daily0am-JST', {
      schedule: events.Schedule.cron({ minute: '0', hour: '15' }),
      targets: [new targets.LambdaFunction(dailyOrganizeDatabaseFunction)]
    })
    new events.Rule(this, 'daily1am-JST', {
      schedule: events.Schedule.cron({ minute: '0', hour: '16'}),
      targets: [new targets.LambdaFunction(transferCollectionHistoryBigqueryFunction)]
    })
  }
}
