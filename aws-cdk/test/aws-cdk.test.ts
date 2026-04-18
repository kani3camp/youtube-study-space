import * as fs from 'node:fs'
import * as os from 'node:os'
import * as path from 'node:path'

import * as cdk from 'aws-cdk-lib'
import { Match, Template } from 'aws-cdk-lib/assertions'

import { AwsCdkStack } from '../lib/aws-cdk-stack'

const createTemplate = () => {
	const app = new cdk.App()
	const stack = new AwsCdkStack(app, 'TestStack')

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
})

// Issue #692: Docker アセット決定性テスト
//
// 目的: `cdk synth` の `dockerImages` キー（= ECR image tag）が、アプリコードと
// 無関係なローカル成果物・ドキュメント・エディタ設定の変更で変動しないことを担保する。
// `system/.dockerignore` が正しく build context から除外している限り、これらの
// 変更は asset fingerprint に寄与しないはず。
describe('Docker asset determinism (issue #692)', () => {
	const SYSTEM_DIR = path.resolve(__dirname, '../../system')

	// `cdk.App` を毎回新しい一時 outdir で synth し、TestStack.assets.json の
	// `dockerImages` キー集合をソート済み配列で返す。
	const synthDockerImageKeys = (): string[] => {
		const outdir = fs.mkdtempSync(path.join(os.tmpdir(), 'cdk-692-'))
		try {
			const app = new cdk.App({ outdir })
			new AwsCdkStack(app, 'TestStack')
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

	// system/ 配下の対象ファイルを一時的に書き換え（または新規作成）して fn を実行し、
	// finally で必ず原状復帰する。
	// - 既存ファイル: tmp 配下に copyFileSync で退避し、終了時に元の場所へコピーし直す。
	//   mode（実行ビット等）は `statSync().mode` をキャプチャして `chmodSync` で復元する。
	//   `system/main` のようなローカル go build 成果物（0o755）が混ざる環境でも、
	//   テスト後に実行不可にならないよう内容だけでなく mode も明示的に戻す。
	// - 存在しなかったパス: 作成 → 削除（新規作成で巻き込んだディレクトリも掃除）。
	const withTemporaryFile = (
		relPath: string,
		content: Buffer | string,
		fn: () => void,
	) => {
		const target = path.join(SYSTEM_DIR, relPath)
		const existed = fs.existsSync(target)
		const createdDirs: string[] = []
		let backupDir: string | null = null
		let backupPath: string | null = null
		let originalMode: number | null = null
		if (existed) {
			backupDir = fs.mkdtempSync(path.join(os.tmpdir(), 'cdk-692-bak-'))
			backupPath = path.join(backupDir, 'backup')
			fs.copyFileSync(target, backupPath)
			originalMode = fs.statSync(target).mode
		} else {
			let dir = path.dirname(target)
			const stopAt = SYSTEM_DIR
			while (!fs.existsSync(dir) && dir.startsWith(stopAt)) {
				createdDirs.push(dir)
				dir = path.dirname(dir)
			}
			fs.mkdirSync(path.dirname(target), { recursive: true })
		}
		const payload =
			typeof content === 'string' ? Buffer.from(content, 'utf-8') : content
		try {
			fs.writeFileSync(target, payload)
			fn()
		} finally {
			try {
				if (existed && backupPath !== null) {
					fs.copyFileSync(backupPath, target)
					if (originalMode !== null) {
						fs.chmodSync(target, originalMode)
					}
				} else {
					if (fs.existsSync(target)) {
						fs.unlinkSync(target)
					}
					for (const dir of createdDirs) {
						if (
							fs.existsSync(dir) &&
							fs.readdirSync(dir).length === 0
						) {
							fs.rmdirSync(dir)
						}
					}
				}
			} finally {
				if (backupDir !== null) {
					fs.rmSync(backupDir, { recursive: true, force: true })
				}
			}
		}
	}

	// 安全弁: `withTemporaryFile` は `finally` でバックアップ / 削除を行うが、
	// テストが途中クラッシュした場合でも worktree が汚染されないよう
	// `afterAll` でも必要最小限の後始末を行えるようにフックだけ用意しておく。
	afterAll(() => {
		// 現状のテストは既存ファイルの内容書き換え + 復元のみで、
		// 新規作成は `withTemporaryFile` 内の try/finally で確実に掃除される。
	})

	let baselineKeys: string[]
	beforeAll(() => {
		baselineKeys = synthDockerImageKeys()
		// Docker アセットが少なくとも 1 つ存在することの sanity check
		expect(baselineKeys.length).toBeGreaterThan(0)
	})

	test('dockerImages keys are identical across two consecutive synths', () => {
		const second = synthDockerImageKeys()
		expect(second).toEqual(baselineKeys)
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
			withTemporaryFile(rel, content, () => {
				const keys = synthDockerImageKeys()
				expect(keys).toEqual(baselineKeys)
			})
		},
	)
})
