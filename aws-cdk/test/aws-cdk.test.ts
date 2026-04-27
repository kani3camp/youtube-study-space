import * as fs from 'node:fs'
import * as os from 'node:os'
import * as path from 'node:path'

import * as cdk from 'aws-cdk-lib'
import { Match, Template } from 'aws-cdk-lib/assertions'

import { AwsCdkStack, type AwsCdkStackProps } from '../lib/aws-cdk-stack'

const REPO_SYSTEM_DIR = path.resolve(__dirname, '../../system')

const createTemplate = (props?: AwsCdkStackProps) => {
	const app = new cdk.App()
	const stack = new AwsCdkStack(app, 'TestStack', props)

	return Template.fromStack(stack)
}

describe('AwsCdkStack', () => {
	const template = createTemplate()
	const allResources = template.toJSON().Resources as Record<
		string,
		{
			Type: string
			Properties?: {
				RetentionInDays?: number
			}
		}
	>

	test('keeps the daily batch schedule invariant', () => {
		template.hasResourceProperties('AWS::Scheduler::Schedule', {
			Name: 'daily-batch-00-00-jst',
			ScheduleExpression: 'cron(0 15 * * ? *)',
			State: 'ENABLED',
			FlexibleTimeWindow: {
				Mode: 'OFF',
			},
			Target: {
				RetryPolicy: {
					MaximumRetryAttempts: 0,
				},
			},
		})
	})

	test('disables retry for all 1 minute rule targets', () => {
		const resources = template.toJSON().Resources as Record<
			string,
			{
				Type: string
				Properties?: {
					ScheduleExpression?: string
					Targets?: Array<{
						RetryPolicy?: {
							MaximumRetryAttempts?: number
						}
					}>
				}
			}
		>
		const oneMinuteRules = Object.values(resources).filter(
			(resource) =>
				resource.Type === 'AWS::Events::Rule' &&
				resource.Properties?.ScheduleExpression === 'rate(1 minute)',
		)
		expect(oneMinuteRules).toHaveLength(1)
		const [oneMinuteRule] = oneMinuteRules
		const targets = oneMinuteRule?.Properties?.Targets

		expect(oneMinuteRule).toBeDefined()
		expect(targets).toBeDefined()
		expect(targets).toHaveLength(2)
		for (const target of targets ?? []) {
			expect(target.RetryPolicy).toEqual({
				MaximumRetryAttempts: 0,
			})
		}
	})

	test('keeps API Gateway protected by an API key', () => {
		template.hasResourceProperties('AWS::ApiGateway::Method', {
			HttpMethod: 'POST',
			ApiKeyRequired: true,
			Integration: {
				Type: 'AWS_PROXY',
			},
		})
		template.hasOutput('ApiEndpointUrl', {})
	})

	test('keeps the Fargate task definition runtime invariant', () => {
		template.hasResourceProperties('AWS::ECS::TaskDefinition', {
			Cpu: '256',
			Memory: '512',
			RuntimePlatform: {
				CpuArchitecture: 'ARM64',
				OperatingSystemFamily: 'LINUX',
			},
		})
	})

	test('exposes the required batch outputs', () => {
		template.hasOutput('BatchClusterArn', {})
		template.hasOutput('DailyBatchTaskDefinitionArn', {})
		template.hasOutput('BatchSecurityGroupId', {})
		template.hasOutput('BatchVpcId', {})
		template.hasOutput('DailyBatchStateMachineArn', {})
		template.hasOutput(
			'BatchPublicSubnetIds',
			Match.objectLike({
				Export: {
					Name: 'BatchPublicSubnetIds',
				},
			}),
		)
	})

	test('keeps all CloudWatch log groups at infinite retention', () => {
		const logGroups = Object.values(allResources).filter(
			(resource) => resource.Type === 'AWS::Logs::LogGroup',
		)

		expect(logGroups.length).toBeGreaterThan(0)
		for (const logGroup of logGroups) {
			expect(logGroup.Properties?.RetentionInDays).toBeUndefined()
		}
	})

	test('defines AlarmEmail template parameter for SNS email backstop', () => {
		const t = createTemplate()
		t.hasParameter('AlarmEmail', { Type: 'String', Default: '' })
	})

	test('subscribes AlarmsTopic to email and Lambda notifier', () => {
		const t = createTemplate()
		const json = t.toJSON() as {
			Conditions?: Record<string, unknown>
			Resources?: Record<
				string,
				{
					Type?: string
					Condition?: string
					Properties?: { Protocol?: string }
				}
			>
		}
		const subs = t.findResources('AWS::SNS::Subscription')
		expect(Object.keys(subs).length).toBe(2)
		expect(json.Conditions).toHaveProperty('HasAlarmEmail')
		t.hasResourceProperties('AWS::SNS::Subscription', {
			Protocol: 'email',
		})
		t.hasResourceProperties('AWS::SNS::Subscription', {
			Protocol: 'lambda',
		})
		const emailSubscription = Object.values(json.Resources ?? {}).find(
			(resource) =>
				resource.Type === 'AWS::SNS::Subscription' &&
				resource.Properties?.Protocol === 'email',
		)
		expect(emailSubscription?.Condition).toBe('HasAlarmEmail')
	})

	test('creates Lambda errors alarms for sns_notify_discord and start_daily_batch', () => {
		const t = createTemplate()
		const alarms = t.findResources('AWS::CloudWatch::Alarm')
		const descriptions = Object.values(alarms).map(
			(a) => (a as { Properties?: { AlarmDescription?: string } }).Properties?.AlarmDescription,
		)
		expect(descriptions).toContain('Lambda sns_notify_discord errors > 0')
		expect(descriptions).toContain('Lambda start_daily_batch errors > 0')
	})

	test('creates error_log_notify_discord Lambda, errors alarm, and six ERROR subscription filters', () => {
		const t = createTemplate()
		const json = t.toJSON() as {
			Resources?: Record<
				string,
				{
					Type?: string
					Properties?: {
						FunctionName?: string
						Principal?: string
					}
				}
			>
		}
		const resources = json.Resources ?? {}
		const lambdaNames = Object.values(resources)
			.filter((r) => r.Type === 'AWS::Lambda::Function')
			.map((r) => r.Properties?.FunctionName)
		expect(lambdaNames).toContain('error_log_notify_discord')

		const filters = Object.values(resources).filter(
			(r) => r.Type === 'AWS::Logs::SubscriptionFilter',
		)
		expect(filters).toHaveLength(6)
		expect(filters.map((filter) => JSON.stringify(filter))).not.toEqual(
			expect.arrayContaining([
				expect.stringContaining('error_log_notify_discord'),
			]),
		)

		const logRetentionCustomResources = Object.values(resources).filter(
			(r) => r.Type === 'Custom::LogRetention',
		)
		expect(logRetentionCustomResources.length).toBeGreaterThanOrEqual(6)

		const logsInvokePerms = Object.values(resources).filter(
			(r) =>
				r.Type === 'AWS::Lambda::Permission' &&
				r.Properties?.Principal === 'logs.amazonaws.com',
		)
		expect(logsInvokePerms.length).toBeGreaterThanOrEqual(3)

		const alarms = t.findResources('AWS::CloudWatch::Alarm')
		const descriptions = Object.values(alarms).map(
			(a) => (a as { Properties?: { AlarmDescription?: string } }).Properties?.AlarmDescription,
		)
		expect(descriptions).toContain('Lambda error_log_notify_discord errors > 0')
	})
})

// Issue #692: Docker アセット決定性テスト
//
// 目的: `cdk synth` の `dockerImages` キー（= ECR image tag）が、アプリコードと
// 無関係なローカル成果物・ドキュメント・エディタ設定の変更で変動しないことを担保する。
// `system/.dockerignore` が正しく build context から除外している限り、これらの
// 変更は asset fingerprint に寄与しないはず。
describe('Docker asset determinism (issue #692)', () => {
	// `cdk.App` を毎回新しい一時 outdir で synth し、TestStack.assets.json の
	// `dockerImages` キー集合をソート済み配列で返す。
	const synthDockerImageKeys = (systemDir = REPO_SYSTEM_DIR): string[] => {
		const outdir = fs.mkdtempSync(path.join(os.tmpdir(), 'cdk-692-'))
		try {
			const app = new cdk.App({ outdir })
			new AwsCdkStack(app, 'TestStack', { systemDir })
			const assembly = app.synth()
			const assetsPath = path.join(
				assembly.directory,
				'TestStack.assets.json',
			)
			const assets = JSON.parse(fs.readFileSync(assetsPath, 'utf-8')) as {
				dockerImages?: Record<string, unknown>
			}
			return Object.keys(assets.dockerImages ?? {}).sort()
		} finally {
			fs.rmSync(outdir, { recursive: true, force: true })
		}
	}

	const withTemporarySystemDir = <T>(fn: (systemDir: string) => T): T => {
		const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'cdk-692-system-'))
		const tempSystemDir = path.join(tempRoot, 'system')
		try {
			fs.cpSync(REPO_SYSTEM_DIR, tempSystemDir, {
				recursive: true,
				preserveTimestamps: true,
			})
			return fn(tempSystemDir)
		} finally {
			fs.rmSync(tempRoot, { recursive: true, force: true })
		}
	}

	// system/ の一時コピー配下だけを書き換え、リポジトリ実ファイルは一切変更しない。
	const writeTemporaryFile = (
		systemDir: string,
		relPath: string,
		content: Buffer | string,
	) => {
		const target = path.join(systemDir, relPath)
		const payload =
			typeof content === 'string' ? Buffer.from(content, 'utf-8') : content
		fs.mkdirSync(path.dirname(target), { recursive: true })
		fs.writeFileSync(target, payload)
	}

	test('dockerImages keys are identical across two consecutive synths', () => {
		withTemporarySystemDir((systemDir) => {
			const baselineKeys = synthDockerImageKeys(systemDir)
			// Docker アセットが少なくとも 1 つ存在することの sanity check
			expect(baselineKeys.length).toBeGreaterThan(0)

			const second = synthDockerImageKeys(systemDir)
			expect(second).toEqual(baselineKeys)
		})
	})

	test.each<{ name: string; rel: string; content: Buffer | string }>([
		{
			name: 'system/main (local go build artifact)',
			rel: 'main',
			content: Buffer.from('local-build-artifact-test'),
		},
		{
			name: 'system/README.md',
			rel: 'README.md',
			content: '# temporary edit for test\n',
		},
		{
			name: 'system/AI_COLLABORATION_GUIDE.md',
			rel: 'AI_COLLABORATION_GUIDE.md',
			content: '# temporary edit for test\n',
		},
		{
			// `**/.cursor` が `.dockerignore` で除外されていることを担保する。
			// 新規作成はエージェントサンドボックス等で EPERM になる環境があるため、
			// 既存ファイル（system/.cursor/rules/specification.mdc）の内容を
			// 一時的に書き換えるアプローチに統一している。
			name: 'system/.cursor/rules/specification.mdc',
			rel: path.join('.cursor', 'rules', 'specification.mdc'),
			content: '// temporary edit for test\n',
		},
		{
			name: 'system/main.go (local-only entrypoint; not built into images)',
			rel: 'main.go',
			content: '// temporary edit for test\n',
		},
	])(
		'dockerImages keys unaffected by changes to $name',
		({ rel, content }) => {
			withTemporarySystemDir((systemDir) => {
				const baselineKeys = synthDockerImageKeys(systemDir)
				expect(baselineKeys.length).toBeGreaterThan(0)

				writeTemporaryFile(systemDir, rel, content)

				const keys = synthDockerImageKeys(systemDir)
				expect(keys).toEqual(baselineKeys)
			})
		},
	)
})
