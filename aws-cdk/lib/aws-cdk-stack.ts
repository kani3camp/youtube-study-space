import * as cdk from "aws-cdk-lib";
import { Construct } from "constructs";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as iam from "aws-cdk-lib/aws-iam";
import { Platform } from "aws-cdk-lib/aws-ecr-assets";
import * as ecr_assets from "aws-cdk-lib/aws-ecr-assets";
import * as ecs from "aws-cdk-lib/aws-ecs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as sfn from "aws-cdk-lib/aws-stepfunctions";
import * as sfn_tasks from "aws-cdk-lib/aws-stepfunctions-tasks";
import * as path from "path";
import { aws_apigateway } from "aws-cdk-lib";
import * as events from "aws-cdk-lib/aws-events";
import * as targets from "aws-cdk-lib/aws-events-targets";
import { PassthroughBehavior } from "aws-cdk-lib/aws-apigateway";
import * as logs from "aws-cdk-lib/aws-logs";
import * as cloudwatch from "aws-cdk-lib/aws-cloudwatch";
import * as sns from "aws-cdk-lib/aws-sns";
import * as subs from "aws-cdk-lib/aws-sns-subscriptions";
import * as cw_actions from "aws-cdk-lib/aws-cloudwatch-actions";
import * as scheduler from "aws-cdk-lib/aws-scheduler";
import * as secretsmanager from "aws-cdk-lib/aws-secretsmanager";

// Docker asset path constants (can be overridden via context in future PRs)
const SYSTEM_DIR = path.join(__dirname, "../../system/");
const DOCKERFILE_LAMBDA = "Dockerfile.lambda";
const DOCKERFILE_FARGATE = "Dockerfile.fargate";

export class AwsCdkStack extends cdk.Stack {
	constructor(scope: Construct, id: string, props?: cdk.StackProps) {
		super(scope, id, props);

		// =========================
		// Secrets Manager
		// =========================
		const openaiApiKeySecret = new secretsmanager.Secret(
			this,
			"YoutubeStudySpaceSecret",
			{
				secretName: "youtube-study-space-secret",
				description: "Youtube Study Space Secret",
			},
		);

		// NOTE: 現状、DynamoDBのテーブルは別途作成しておく必要がある
		const dynamoDBAccessPolicy = new iam.PolicyStatement({
			actions: ["dynamodb:GetItem"],
			effect: iam.Effect.ALLOW,
			resources: ["arn:aws:dynamodb:*:*:table/secrets"],
		});

		// =========================
		// ECS/Fargate: Daily Batch
		// =========================
		// VPC: Public Subnet のみ、NATなし（コスト最小）
		const vpc = new ec2.Vpc(this, "BatchVpc", {
			natGateways: 0,
			subnetConfiguration: [
				{
					name: "public",
					subnetType: ec2.SubnetType.PUBLIC,
				},
			],
		});

		// DynamoDB Gateway VPC Endpoint for secure, cost-effective access
		// Note: This VPC uses only Public Subnets (no NAT). Gateway endpoint attaches to route tables
		// in these public subnets and enables private DynamoDB access without NAT egress.
		vpc.addGatewayEndpoint("DynamoDbEndpoint", {
			service: ec2.GatewayVpcEndpointAwsService.DYNAMODB,
			// public subnets are fine; gateway endpoints are attached to the route tables
			// associatedRoutes can be left default to all route tables in the VPC
		});

		// 最小限のegressのみ許可するSG
		const batchSecurityGroup = new ec2.SecurityGroup(
			this,
			"BatchSecurityGroup",
			{
				vpc,
				allowAllOutbound: false,
				description: "Minimal egress for Fargate batch",
			},
		);
		// HTTPS (外部API/GCP等)
		batchSecurityGroup.addEgressRule(
			ec2.Peer.anyIpv4(),
			ec2.Port.tcp(443),
			"HTTPS to internet",
		);
		// VPC DNS リゾルバ (169.254.169.253) への TCP/UDP 53
		batchSecurityGroup.addEgressRule(
			ec2.Peer.ipv4("169.254.169.253/32"),
			ec2.Port.tcp(53),
			"DNS TCP to VPC resolver",
		);
		batchSecurityGroup.addEgressRule(
			ec2.Peer.ipv4("169.254.169.253/32"),
			ec2.Port.udp(53),
			"DNS UDP to VPC resolver",
		);
		// ECS Task メタデータ/credential (169.254.170.2:80)
		batchSecurityGroup.addEgressRule(
			ec2.Peer.ipv4("169.254.170.2/32"),
			ec2.Port.tcp(80),
			"ECS task metadata/credentials",
		);

		const cluster = new ecs.Cluster(this, "BatchCluster", { vpc });

		const batchLogGroup = new logs.LogGroup(this, "BatchLogGroup", {
			retention: logs.RetentionDays.ONE_MONTH,
		});

		const batchImageAsset = new ecr_assets.DockerImageAsset(
			this,
			"BatchImage",
			{
				directory: SYSTEM_DIR,
				file: DOCKERFILE_FARGATE,
				platform: Platform.LINUX_ARM64,
			},
		);

		const taskDefinition = new ecs.FargateTaskDefinition(
			this,
			"DailyBatchTaskDefinition",
			{
				cpu: 256,
				memoryLimitMiB: 512,
				runtimePlatform: {
					cpuArchitecture: ecs.CpuArchitecture.ARM64,
					operatingSystemFamily: ecs.OperatingSystemFamily.LINUX,
				},
			},
		);
		// DynamoDB secrets テーブルへのアクセス付与
		taskDefinition.taskRole.addToPrincipalPolicy(dynamoDBAccessPolicy);

		const batchContainer = taskDefinition.addContainer("daily-batch", {
			image: ecs.ContainerImage.fromDockerImageAsset(batchImageAsset),
			logging: ecs.LogDrivers.awsLogs({
				logGroup: batchLogGroup,
				streamPrefix: "daily-batch",
			}),
			environment: {
				// ECS/Fargate でも AWS_REGION は基本入るが、念のため DEFAULT もセット
				AWS_REGION: cdk.Stack.of(this).region,
				AWS_DEFAULT_REGION: cdk.Stack.of(this).region,
			},
		});

		// SNS topic for CloudWatch alarms and subscription to Discord notify Lambda
		const alarmsTopic = new sns.Topic(this, "AlarmsTopic", {
			displayName: "youtube-study-space-alarms",
		});
		// Unified SNS consumer Lambda for all infra/app alerts
		const snsNotifyDiscordFunction = new lambda.DockerImageFunction(
			this,
			"sns_notify_discord",
			{
				functionName: "sns_notify_discord",
				code: lambda.DockerImageCode.fromImageAsset(SYSTEM_DIR, {
					file: DOCKERFILE_LAMBDA,
					buildArgs: { HANDLER: "main" },
					platform: Platform.LINUX_AMD64,
					entrypoint: ["/app/sns_notify_discord"],
				}),
				timeout: cdk.Duration.seconds(30),
				reservedConcurrentExecutions: 1,
			},
		);
		(snsNotifyDiscordFunction.role as iam.Role).addToPolicy(
			dynamoDBAccessPolicy,
		);
		alarmsTopic.addSubscription(
			new subs.LambdaSubscription(snsNotifyDiscordFunction),
		);

		// Helper to create a common Lambda Errors>0 alarm wired to SNS
		const createLambdaErrorAlarm = (
			fn: lambda.FunctionBase,
			id: string,
			description: string,
		) => {
			const alarm = new cloudwatch.Alarm(this, id, {
				metric: fn.metricErrors({
					statistic: "sum",
					period: cdk.Duration.minutes(5),
				}),
				threshold: 0,
				evaluationPeriods: 1,
				comparisonOperator:
					cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
				treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
				alarmDescription: description,
			});
			alarm.addAlarmAction(new cw_actions.SnsAction(alarmsTopic));
			return alarm;
		};

		// 参照用の出力（後続PRでStep Functionsから使用）
		new cdk.CfnOutput(this, "BatchClusterArn", {
			value: cluster.clusterArn,
			exportName: "BatchClusterArn",
		});
		new cdk.CfnOutput(this, "DailyBatchTaskDefinitionArn", {
			value: taskDefinition.taskDefinitionArn,
			exportName: "DailyBatchTaskDefinitionArn",
		});
		new cdk.CfnOutput(this, "BatchSecurityGroupId", {
			value: batchSecurityGroup.securityGroupId,
			exportName: "BatchSecurityGroupId",
		});
		const publicSubnetIds = vpc.selectSubnets({
			subnetType: ec2.SubnetType.PUBLIC,
		}).subnetIds;
		new cdk.CfnOutput(this, "BatchPublicSubnetIds", {
			value: cdk.Fn.join(",", publicSubnetIds),
			exportName: "BatchPublicSubnetIds",
		});
		new cdk.CfnOutput(this, "BatchVpcId", {
			value: vpc.vpcId,
			exportName: "BatchVpcId",
		});

		// =========================
		// Step Functions: Daily Batch Orchestration
		// =========================
		// RunTask.sync で Fargate タスクを直列実行（JOB=reset → update-rp → transfer-bq）
		const runTaskCommon: sfn_tasks.EcsRunTaskProps = {
			cluster: cluster,
			taskDefinition: taskDefinition,
			launchTarget: new sfn_tasks.EcsFargateLaunchTarget({
				platformVersion: ecs.FargatePlatformVersion.LATEST,
			}),
			assignPublicIp: true,
			securityGroups: [batchSecurityGroup],
			resultPath: sfn.JsonPath.DISCARD,
			integrationPattern: sfn.IntegrationPattern.RUN_JOB,
		};

		const resetDailyTotalTask = new sfn_tasks.EcsRunTask(
			this,
			"reset-daily-total",
			{
				...runTaskCommon,
				containerOverrides: [
					{
						containerDefinition: batchContainer,
						environment: [{ name: "JOB", value: "reset-daily-total" }],
					},
				],
			},
		);
		const updateRpTask = new sfn_tasks.EcsRunTask(this, "update-rp", {
			...runTaskCommon,
			containerOverrides: [
				{
					containerDefinition: batchContainer,
					environment: [{ name: "JOB", value: "update-rp" }],
				},
			],
		});
		const transferBqTask = new sfn_tasks.EcsRunTask(this, "transfer-bq", {
			...runTaskCommon,
			containerOverrides: [
				{
					containerDefinition: batchContainer,
					environment: [{ name: "JOB", value: "transfer-bq" }],
				},
			],
		});

		// Manual-run tasks must be separate instances (states cannot be reused across graphs)
		const manualResetDailyTotalTask = new sfn_tasks.EcsRunTask(
			this,
			"manual-reset-daily-total",
			{
				...runTaskCommon,
				containerOverrides: [
					{
						containerDefinition: batchContainer,
						environment: [{ name: "JOB", value: "reset-daily-total" }],
					},
				],
			},
		);
		const manualUpdateRpTask = new sfn_tasks.EcsRunTask(
			this,
			"manual-update-rp",
			{
				...runTaskCommon,
				containerOverrides: [
					{
						containerDefinition: batchContainer,
						environment: [{ name: "JOB", value: "update-rp" }],
					},
				],
			},
		);
		const manualTransferBqTask = new sfn_tasks.EcsRunTask(
			this,
			"manual-transfer-bq",
			{
				...runTaskCommon,
				containerOverrides: [
					{
						containerDefinition: batchContainer,
						environment: [{ name: "JOB", value: "transfer-bq" }],
					},
				],
			},
		);

		// 日付境界ずれ対策として 15 秒待ってから開始
		const wait15s = new sfn.Wait(this, "wait-00:00:15", {
			time: sfn.WaitTime.duration(cdk.Duration.seconds(15)),
		});

		const notifyOnFailure = new sfn_tasks.SnsPublish(
			this,
			"notify-on-failure-sns",
			{
				topic: alarmsTopic,
				message: sfn.TaskInput.fromObject({
					workflow: "daily-batch",
					stateName: sfn.JsonPath.stringAt("$$.State.Name"),
					executionArn: sfn.JsonPath.stringAt("$$.Execution.Id"),
					error: sfn.JsonPath.stringAt("$.Error"),
					cause: sfn.JsonPath.stringAt("$.Cause"),
				}),
				subject: "daily-batch failed",
				resultPath: sfn.JsonPath.DISCARD,
			},
		);

		// 手動実行用は別グラフになるため、各グラフ専用のSNS通知ステートを定義して接続する

		// Execute all three sequentially but continue on failure (each task has local catch → notify → continue)
		const definition = sfn.Chain.start(wait15s)
			.next(
				resetDailyTotalTask.addCatch(notifyOnFailure, {
					resultPath: sfn.JsonPath.DISCARD,
				}),
			)
			.next(
				updateRpTask.addCatch(notifyOnFailure, {
					resultPath: sfn.JsonPath.DISCARD,
				}),
			)
			.next(
				transferBqTask.addCatch(notifyOnFailure, {
					resultPath: sfn.JsonPath.DISCARD,
				}),
			);

		const dailyBatchStateMachine = new sfn.StateMachine(
			this,
			"daily-batch-sfn",
			{
				definitionBody: sfn.DefinitionBody.fromChainable(definition),
				tracingEnabled: false,
				logs: {
					destination: new logs.LogGroup(this, "DailyBatchSfnLogs", {
						retention: logs.RetentionDays.ONE_MONTH,
					}),
					level: sfn.LogLevel.ERROR,
				},
				timeout: cdk.Duration.hours(3),
			},
		);

		// CloudWatch alarm: Step Functions ExecutionsFailed > 0
		const failedMetric = dailyBatchStateMachine.metricFailed({
			statistic: "sum",
			period: cdk.Duration.minutes(5),
		});
		const sfnFailedAlarm = new cloudwatch.Alarm(this, "DailyBatchFailedAlarm", {
			metric: failedMetric,
			threshold: 0,
			evaluationPeriods: 1,
			comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_THRESHOLD,
			treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
			alarmDescription: "Daily batch Step Functions failed executions > 0",
		});
		sfnFailedAlarm.addAlarmAction(new cw_actions.SnsAction(alarmsTopic));

		new cdk.CfnOutput(this, "DailyBatchStateMachineArn", {
			value: dailyBatchStateMachine.stateMachineArn,
			exportName: "DailyBatchStateMachineArn",
		});

		// Manual one-off runners (no JSON input): 3 dedicated state machines
		const manualResetNotify = new sfn_tasks.SnsPublish(
			this,
			"manual-reset-notify-on-failure",
			{
				topic: alarmsTopic,
				message: sfn.TaskInput.fromObject({
					workflow: "manual-reset-daily-total",
					stateName: sfn.JsonPath.stringAt("$$.State.Name"),
					executionArn: sfn.JsonPath.stringAt("$$.Execution.Id"),
					error: sfn.JsonPath.stringAt("$.Error"),
					cause: sfn.JsonPath.stringAt("$.Cause"),
				}),
				subject: "manual-reset-daily-total failed",
				resultPath: sfn.JsonPath.DISCARD,
			},
		);
		const manualResetDefinition = manualResetDailyTotalTask.addCatch(
			manualResetNotify,
			{ resultPath: sfn.JsonPath.DISCARD },
		);
		const manualResetDailyTotalSfn = new sfn.StateMachine(
			this,
			"manual-reset-daily-total-sfn",
			{
				definitionBody: sfn.DefinitionBody.fromChainable(manualResetDefinition),
				tracingEnabled: false,
				logs: {
					destination: new logs.LogGroup(this, "ManualResetDailyTotalSfnLogs", {
						retention: logs.RetentionDays.ONE_MONTH,
					}),
					level: sfn.LogLevel.ERROR,
				},
				timeout: cdk.Duration.hours(3),
			},
		);
		new cdk.CfnOutput(this, "ManualResetDailyTotalStateMachineArn", {
			value: manualResetDailyTotalSfn.stateMachineArn,
			exportName: "ManualResetDailyTotalStateMachineArn",
		});

		const manualUpdateNotify = new sfn_tasks.SnsPublish(
			this,
			"manual-update-notify-on-failure",
			{
				topic: alarmsTopic,
				message: sfn.TaskInput.fromObject({
					workflow: "manual-update-rp",
					stateName: sfn.JsonPath.stringAt("$$.State.Name"),
					executionArn: sfn.JsonPath.stringAt("$$.Execution.Id"),
					error: sfn.JsonPath.stringAt("$.Error"),
					cause: sfn.JsonPath.stringAt("$.Cause"),
				}),
				subject: "manual-update-rp failed",
				resultPath: sfn.JsonPath.DISCARD,
			},
		);
		const manualUpdateDefinition = manualUpdateRpTask.addCatch(
			manualUpdateNotify,
			{ resultPath: sfn.JsonPath.DISCARD },
		);
		const manualUpdateRpSfn = new sfn.StateMachine(
			this,
			"manual-update-rp-sfn",
			{
				definitionBody: sfn.DefinitionBody.fromChainable(
					manualUpdateDefinition,
				),
				tracingEnabled: false,
				logs: {
					destination: new logs.LogGroup(this, "ManualUpdateRpSfnLogs", {
						retention: logs.RetentionDays.ONE_MONTH,
					}),
					level: sfn.LogLevel.ERROR,
				},
				timeout: cdk.Duration.hours(3),
			},
		);
		new cdk.CfnOutput(this, "ManualUpdateRpStateMachineArn", {
			value: manualUpdateRpSfn.stateMachineArn,
			exportName: "ManualUpdateRpStateMachineArn",
		});

		const manualTransferNotify = new sfn_tasks.SnsPublish(
			this,
			"manual-transfer-notify-on-failure",
			{
				topic: alarmsTopic,
				message: sfn.TaskInput.fromObject({
					workflow: "manual-transfer-bq",
					stateName: sfn.JsonPath.stringAt("$$.State.Name"),
					executionArn: sfn.JsonPath.stringAt("$$.Execution.Id"),
					error: sfn.JsonPath.stringAt("$.Error"),
					cause: sfn.JsonPath.stringAt("$.Cause"),
				}),
				subject: "manual-transfer-bq failed",
				resultPath: sfn.JsonPath.DISCARD,
			},
		);
		const manualTransferDefinition = manualTransferBqTask.addCatch(
			manualTransferNotify,
			{ resultPath: sfn.JsonPath.DISCARD },
		);
		const manualTransferBqSfn = new sfn.StateMachine(
			this,
			"manual-transfer-bq-sfn",
			{
				definitionBody: sfn.DefinitionBody.fromChainable(
					manualTransferDefinition,
				),
				tracingEnabled: false,
				logs: {
					destination: new logs.LogGroup(this, "ManualTransferBqSfnLogs", {
						retention: logs.RetentionDays.ONE_MONTH,
					}),
					level: sfn.LogLevel.ERROR,
				},
				timeout: cdk.Duration.hours(3),
			},
		);
		new cdk.CfnOutput(this, "ManualTransferBqStateMachineArn", {
			value: manualTransferBqSfn.stateMachineArn,
			exportName: "ManualTransferBqStateMachineArn",
		});

		// EventBridge Scheduler: 00:00 JST (= UTC 15:00) → start_daily_batch Lambda
		const schedulerRole = new iam.Role(this, "DailyBatchSchedulerRole", {
			assumedBy: new iam.ServicePrincipal("scheduler.amazonaws.com"),
		});
		// start_daily_batch Lambda to start SFN with idempotent name
		const startDailyBatchFunction = new lambda.DockerImageFunction(
			this,
			"start_daily_batch",
			{
				functionName: "start_daily_batch",
				code: lambda.DockerImageCode.fromImageAsset(SYSTEM_DIR, {
					file: DOCKERFILE_LAMBDA,
					buildArgs: { HANDLER: "main" },
					platform: Platform.LINUX_AMD64,
					entrypoint: ["/app/start_daily_batch"],
				}),
				timeout: cdk.Duration.seconds(15),
				reservedConcurrentExecutions: 1,
				environment: {
					STATE_MACHINE_ARN: dailyBatchStateMachine.stateMachineArn,
				},
			},
		);
		startDailyBatchFunction.grantInvoke(schedulerRole);
		dailyBatchStateMachine.grantStartExecution(startDailyBatchFunction);

		new scheduler.CfnSchedule(this, "DailyBatchScheduler", {
			flexibleTimeWindow: { mode: "OFF" },
			scheduleExpression: "cron(0 15 * * ? *)",
			target: {
				arn: startDailyBatchFunction.functionArn,
				roleArn: schedulerRole.roleArn,
			},
			name: "daily-batch-00-00-jst",
			description: "Start daily batch SFN with idempotent name",
			state: "ENABLED",
		});

		// Lambda function
		const setDesiredMaxSeatsFunction = new lambda.DockerImageFunction(
			this,
			"set_desired_max_seats",
			{
				functionName: "set_desired_max_seats",
				code: lambda.DockerImageCode.fromImageAsset(SYSTEM_DIR, {
					file: DOCKERFILE_LAMBDA,
					buildArgs: {
						HANDLER: "main",
					},
					platform: Platform.LINUX_AMD64,
					entrypoint: ["/app/set_desired_max_seats"],
				}),
				timeout: cdk.Duration.seconds(20),
				reservedConcurrentExecutions: undefined,
			},
		);
		(setDesiredMaxSeatsFunction.role as iam.Role).addToPolicy(
			dynamoDBAccessPolicy,
		);
		createLambdaErrorAlarm(
			setDesiredMaxSeatsFunction,
			"SetDesiredMaxSeatsErrorsAlarm",
			"Lambda set_desired_max_seats errors > 0",
		);

		const youtubeOrganizeDatabaseFunction = new lambda.DockerImageFunction(
			this,
			"youtube_organize_database",
			{
				functionName: "youtube_organize_database",
				code: lambda.DockerImageCode.fromImageAsset(SYSTEM_DIR, {
					file: DOCKERFILE_LAMBDA,
					buildArgs: {
						HANDLER: "main",
					},
					platform: Platform.LINUX_AMD64,
					entrypoint: ["/app/youtube_organize_database"],
				}),
				timeout: cdk.Duration.seconds(50),
				reservedConcurrentExecutions: 1,
			},
		);
		(youtubeOrganizeDatabaseFunction.role as iam.Role).addToPolicy(
			dynamoDBAccessPolicy,
		);
		createLambdaErrorAlarm(
			youtubeOrganizeDatabaseFunction,
			"YoutubeOrganizeDatabaseErrorsAlarm",
			"Lambda youtube_organize_database errors > 0",
		);

		const checkLiveStreamStatusFunction = new lambda.DockerImageFunction(
			this,
			"check_live_stream_status",
			{
				functionName: "check_live_stream_status",
				code: lambda.DockerImageCode.fromImageAsset(SYSTEM_DIR, {
					file: DOCKERFILE_LAMBDA,
					buildArgs: {
						HANDLER: "main",
					},
					platform: Platform.LINUX_AMD64,
					entrypoint: ["/app/check_live_stream_status"],
				}),
				timeout: cdk.Duration.seconds(20),
				reservedConcurrentExecutions: undefined,
			},
		);
		(checkLiveStreamStatusFunction.role as iam.Role).addToPolicy(
			dynamoDBAccessPolicy,
		);
		createLambdaErrorAlarm(
			checkLiveStreamStatusFunction,
			"CheckLiveStreamStatusErrorsAlarm",
			"Lambda check_live_stream_status errors > 0",
		);

		const updateWorkNameTrendFunction = new lambda.DockerImageFunction(
			this,
			"update_work_name_trend",
			{
				functionName: "update_work_name_trend",
				code: lambda.DockerImageCode.fromImageAsset(SYSTEM_DIR, {
					file: DOCKERFILE_LAMBDA,
					buildArgs: {
						HANDLER: "main",
					},
					platform: Platform.LINUX_AMD64,
					entrypoint: ["/app/update_work_name_trend"],
				}),
				timeout: cdk.Duration.minutes(5),
				reservedConcurrentExecutions: 1,
				environment: {
					SECRET_NAME: openaiApiKeySecret.secretName,
				},
			},
		);
		openaiApiKeySecret.grantRead(updateWorkNameTrendFunction);
		(updateWorkNameTrendFunction.role as iam.Role).addToPolicy(
			dynamoDBAccessPolicy,
		);
		createLambdaErrorAlarm(
			updateWorkNameTrendFunction,
			"UpdateWorkNameTrendErrorsAlarm",
			"Lambda update_work_name_trend errors > 0",
		);

		// API Gateway用ロググループ
		const restApiLogAccessLogGroup = new logs.LogGroup(
			this,
			"RestApiLogAccessLogGroup",
			{
				retention: logs.RetentionDays.INFINITE,
			},
		);

		// API Gateway
		const restApi = new aws_apigateway.RestApi(
			this,
			"youtube-study-space-rest-api",
			{
				deployOptions: {
					stageName: "default",
					dataTraceEnabled: true,
					loggingLevel: aws_apigateway.MethodLoggingLevel.INFO,
					accessLogDestination: new aws_apigateway.LogGroupLogDestination(
						restApiLogAccessLogGroup,
					),
					accessLogFormat: aws_apigateway.AccessLogFormat.clf(),
				},
				restApiName: "youtube-study-space-rest-api",
				defaultMethodOptions: { apiKeyRequired: true },
				defaultCorsPreflightOptions: {
					allowOrigins: aws_apigateway.Cors.ALL_ORIGINS,
					allowMethods: aws_apigateway.Cors.ALL_METHODS,
					allowHeaders: aws_apigateway.Cors.DEFAULT_HEADERS,
					statusCode: 200,
				},
				cloudWatchRole: true,
			},
		);

		const apiKey = restApi.addApiKey("youtube-study-space-api-key", {
			apiKeyName: `youtube-study-space-api-key`,
		});

		// const plan = restApi.addUsagePlan("UsagePlan");
		const plan = restApi.addUsagePlan("UsagePlan", {
			name: `youtube-study-space`,
		});
		plan.addApiKey(apiKey);
		plan.addApiStage({ stage: restApi.deploymentStage });

		const apiSetDesiredMaxSeats = restApi.root.addResource(
			"set_desired_max_seats",
		);
		apiSetDesiredMaxSeats.addMethod(
			"POST",
			new aws_apigateway.LambdaIntegration(setDesiredMaxSeatsFunction, {
				passthroughBehavior: PassthroughBehavior.WHEN_NO_MATCH,
			}),
			{
				methodResponses: [
					{
						statusCode: "200",
						responseModels: {
							"application/json": aws_apigateway.Model.EMPTY_MODEL,
						},
						responseParameters: {
							"method.response.header.Access-Control-Allow-Origin": true,
						},
					},
				],
			},
		);

		// APIエンドポイントURLを出力
		new cdk.CfnOutput(this, "ApiEndpointUrl", {
			value: restApi.url,
			description: "The URL of the API Gateway endpoint",
			exportName: "YoutubeStudySpaceApiEndpointUrl",
		});

		// EventBridge
		new events.Rule(this, "1minute", {
			schedule: events.Schedule.rate(cdk.Duration.minutes(1)),
			targets: [
				new targets.LambdaFunction(youtubeOrganizeDatabaseFunction),
				new targets.LambdaFunction(checkLiveStreamStatusFunction),
			],
		});

		new events.Rule(this, "5minutes", {
			schedule: events.Schedule.rate(cdk.Duration.minutes(5)),
			targets: [new targets.LambdaFunction(updateWorkNameTrendFunction)],
		});
	}
}
