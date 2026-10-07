import * as fs from 'node:fs'
import * as os from 'node:os'
import * as path from 'node:path'
import { spawnSync } from 'node:child_process'
import * as cdk from 'aws-cdk-lib'
import { AwsCdkStack } from '../lib/aws-cdk-stack'
import { prepareBatchImageCandidate, writeBatchImageCandidate, type BatchImageCandidateInput } from '../lib/batch-image-candidate'

const app = new cdk.App()
const stack = new AwsCdkStack(app, 'AwsCdkStack')
const assembly = app.synth()
const baseline = assembly.getStackArtifact(stack.artifactId).template
const taskLogicalId = Object.keys(baseline.Resources).find((id) => baseline.Resources[id].Type === 'AWS::ECS::TaskDefinition')!
const managedTaskArn = `arn:aws:ecs:ap-northeast-1:657533259235:task-definition/${baseline.Resources[taskLogicalId].Properties.Family}:7`
const image = baseline.Resources[taskLogicalId].Properties.ContainerDefinitions[0].Image['Fn::Sub'] as string
const repository = image.replace(/\$\{AWS::AccountId\}/g, '657533259235')
	.replace(/\$\{AWS::Region\}/g, 'ap-northeast-1').replace(/\$\{AWS::URLSuffix\}/g, 'amazonaws.com').split(':')[0]
// Synthetic digest and stack receipt. These are not live evidence.
const input = (): BatchImageCandidateInput => ({
	baselineTemplate: JSON.parse(JSON.stringify(baseline)),
	stackArtifact: JSON.parse(JSON.stringify(assembly.manifest.artifacts![stack.artifactId])),
	stackReceipt: {
		StackId: 'arn:aws:cloudformation:ap-northeast-1:657533259235:stack/AwsCdkStack/synthetic-receipt',
		StackStatus: 'UPDATE_COMPLETE',
		RoleARN: 'arn:aws:iam::657533259235:role/cdk-hnb659fds-cfn-exec-role-657533259235-ap-northeast-1',
		EnableTerminationProtection: true,
		DailyBatchTaskDefinitionArn: managedTaskArn,
	},
	taskLogicalId,
	currentTaskDefinitionArn: managedTaskArn,
	candidateImage: `${repository}@sha256:${'a'.repeat(64)}`,
	previousImage: `${repository}@sha256:${'b'.repeat(64)}`,
})
const taskContainer = (template: any) => template.Resources[taskLogicalId].Properties.ContainerDefinitions[0]

describe('offline dev batch image candidate', () => {
	test('changes only the image and retains all deployed fields for deploy and pinned rollback', () => {
		const source = input()
		const original = JSON.parse(JSON.stringify(source))
		const prepared = prepareBatchImageCandidate(source)
		expect(taskContainer(prepared.candidate).Image).toBe(source.candidateImage)
		expect(taskContainer(prepared.rollback).Image).toBe(source.previousImage)
		for (const template of [prepared.candidate, prepared.rollback]) {
			taskContainer(template).Image = taskContainer(source.baselineTemplate).Image
			expect(template).toEqual(source.baselineTemplate)
		}
		expect(prepared.original).toEqual(source.baselineTemplate)
		expect(prepared.review.revisionInvariantResourceReferences).toHaveLength(4)
		expect(prepared.review.revisionSensitiveResourceReferenceCount).toBe(0)
		expect(source).toEqual(original)
	})

	test('accounts for all six current source jobs even though state machines are unchanged', () => {
		const prepared = prepareBatchImageCandidate(input())
		expect(prepared.review.templateConsumers).toHaveLength(6)
		expect(new Set(prepared.review.templateConsumers.map((c) => c.stateMachineLogicalId)).size).toBe(4)
		for (const job of ['reset-daily-total', 'update-rp', 'transfer-bq']) {
			expect(prepared.review.templateConsumers.filter((c) => c.job === job)).toHaveLength(2)
		}
	})

	test('discovers additional nested family consumers instead of fixing the impact at six', () => {
		const source = input()
		const family = source.baselineTemplate.Resources[taskLogicalId].Properties.Family
		source.baselineTemplate.Resources.NestedRunner = {
			Type: 'AWS::StepFunctions::StateMachine', Properties: {
				Definition: { States: { Parallel: { Type: 'Parallel', Branches: [{ States: {
					Extra: { Type: 'Task', Resource: 'arn:aws:states:::ecs:runTask.sync', Parameters: {
						TaskDefinition: family, Overrides: { ContainerOverrides: [{ Name: 'daily-batch',
							Environment: [{ Name: 'JOB', Value: 'transfer-bq' }] }] },
					} },
				} }] } } },
			},
		}
		expect(prepareBatchImageCandidate(source).review.templateConsumers).toHaveLength(7)
		source.baselineTemplate.Resources.NestedRunner.Properties.Definition.States.Parallel.Branches[0]
			.States.Extra.Parameters.TaskDefinition += ':65'
		expect(() => prepareBatchImageCandidate(source)).toThrow('unmapped ECS task reference')
		const extra = source.baselineTemplate.Resources.NestedRunner.Properties.Definition.States.Parallel.Branches[0].States.Extra
		extra.Parameters.TaskDefinition = family
		extra.Resource = { 'Fn::GetAtt': ['UnknownRunner', 'Arn'] }
		expect(() => prepareBatchImageCandidate(source)).toThrow('unmapped task resource')
	})

	test.each(['candidateImage', 'previousImage'] as const)('rejects mutable, foreign-account or different repository %s', (field) => {
		for (const uri of [
			`${repository}:latest`,
			`${repository.replace('657533259235', '000000000000')}@sha256:${'a'.repeat(64)}`,
			`${repository}-other@sha256:${'a'.repeat(64)}`,
		]) {
			const source = input()
			source[field] = uri
			expect(() => prepareBatchImageCandidate(source)).toThrow(/immutable/)
		}
	})

	test('rejects an unchanged candidate and a baseline digest inconsistent with rollback', () => {
		const source = input()
		source.candidateImage = source.previousImage
		expect(() => prepareBatchImageCandidate(source)).toThrow('candidate must change image')
		source.candidateImage = `${repository}@sha256:${'c'.repeat(64)}`
		taskContainer(source.baselineTemplate).Image = `${repository}@sha256:${'d'.repeat(64)}`
		expect(() => prepareBatchImageCandidate(source)).toThrow('baseline digest mismatch')
	})

	test('rejects a current latest revision outside CDK ownership and indirect resource changes', () => {
		const source = input()
		source.currentTaskDefinitionArn = managedTaskArn.replace(/:7$/, ':8')
		expect(() => prepareBatchImageCandidate(source)).toThrow('current task must match CDK-owned revision')
		for (const ref of [{ Ref: taskLogicalId }, { 'Fn::GetAtt': [taskLogicalId, 'Arn'] },
			{ 'Fn::Sub': `arn-prefix-${'${' + taskLogicalId + '}'}-suffix` },
			{ 'Fn::Select': [6, { 'Fn::Split': [':', { Ref: taskLogicalId }] }] },
			{ 'Fn::Select': [0, { 'Fn::Split': ['_', { Ref: taskLogicalId }] }] }]) {
			const candidate = input()
			candidate.baselineTemplate.Resources.IndirectPolicy = {
				Type: 'AWS::IAM::Policy', Properties: { PolicyDocument: { Statement: [{ Resource: ref }] } },
			}
			expect(() => prepareBatchImageCandidate(candidate)).toThrow(/dependent resource would change|unprovable task dependency/)
		}
	})

	test('rejects production, unstable stacks, changed execution role and parameter overrides', () => {
		const mutations: Array<(s: BatchImageCandidateInput) => void> = [
			(s) => { s.stackReceipt.StackId = s.stackReceipt.StackId.replace('657533259235', '652333062396') },
			(s) => { s.stackReceipt.StackStatus = 'UPDATE_IN_PROGRESS' },
			(s) => { s.stackReceipt.RoleARN += '-other' },
			(s) => { (s.stackArtifact as any).environment = 'aws://652333062396/ap-northeast-1' },
			(s) => { (s.stackArtifact.properties as any).parameters = { GoogleCloudProject: 'other' } },
		]
		for (const mutate of mutations) {
			const source = input()
			mutate(source)
			expect(() => prepareBatchImageCandidate(source)).toThrow()
		}
	})

	test('rejects changed batch runtime, ambiguous containers and unknown definitions', () => {
		const mutations: Array<(s: BatchImageCandidateInput) => void> = [
			(s) => { s.baselineTemplate.Resources[taskLogicalId].Properties.RuntimePlatform.CpuArchitecture = 'X86_64' },
			(s) => { delete s.baselineTemplate.Resources[taskLogicalId].Properties.Family },
			(s) => { s.baselineTemplate.Resources[taskLogicalId].Properties.ContainerDefinitions.push({ Name: 'sidecar' }) },
			(s) => { s.baselineTemplate.Transform = 'AWS::Serverless-2016-10-31' },
			(s) => { s.baselineTemplate.Resources.Unmapped = { Type: 'AWS::StepFunctions::StateMachine', Properties: { DefinitionS3Location: { Bucket: 'synthetic' } } } },
		]
		for (const mutate of mutations) {
			const source = input()
			mutate(source)
			expect(() => prepareBatchImageCandidate(source)).toThrow()
		}
	})

	test('emits readable CDK assemblies with existing roles, termination protection, and zero publishable assets', () => {
		const parent = fs.mkdtempSync(path.join(os.tmpdir(), 'batch-candidate-'))
		const previousUmask = process.umask(0o022)
		try {
			const out = path.join(parent, 'private')
			const source = input()
			writeBatchImageCandidate(source, out)
			for (const phase of ['candidate', 'rollback']) {
				const dir = path.join(out, phase)
				const result = new cdk.cx_api.CloudAssembly(dir)
				expect(result.stacks).toHaveLength(1)
				expect(result.artifacts.filter((a) => a.manifest.type === cdk.cloud_assembly_schema.ArtifactType.ASSET_MANIFEST)).toHaveLength(0)
				const artifact = result.stacks[0].manifest
				expect(artifact.environment).toBe('aws://657533259235/ap-northeast-1')
				expect(artifact.properties).toMatchObject({
					cloudFormationExecutionRoleArn: (source.stackArtifact.properties as any).cloudFormationExecutionRoleArn,
					assumeRoleArn: (source.stackArtifact.properties as any).assumeRoleArn,
					terminationProtection: true,
				})
				expect((artifact.properties as any).stackTemplateAssetObjectUrl).toBeUndefined()
				expect(artifact.dependencies).toBeUndefined()
				for (const name of fs.readdirSync(dir)) expect(fs.statSync(path.join(dir, name)).mode & 0o077).toBe(0)
			}
			expect(() => writeBatchImageCandidate(source, out)).toThrow()
		} finally {
			process.umask(previousUmask)
			fs.rmSync(parent, { recursive: true, force: true })
		}
	})

	test('CLI fails with redacted output for public or symlink input', () => {
		const parent = fs.mkdtempSync(path.join(os.tmpdir(), 'batch-candidate-cli-'))
		try {
			const file = path.join(parent, 'private-value.json')
			fs.writeFileSync(file, JSON.stringify(input()), { mode: 0o644 })
			fs.chmodSync(file, 0o644)
			const symlink = path.join(parent, 'link.json')
			fs.symlinkSync(file, symlink)
			for (const inputPath of [file, symlink]) {
				const run = spawnSync(process.execPath, [path.resolve(__dirname, '../bin/prepare-batch-image-candidate.js'),
					'--input', inputPath, '--out', path.join(parent, 'out')], { encoding: 'utf8' })
				expect(run.status).toBe(1)
				expect(run.stdout).toBe('')
				expect(run.stderr).toBe('STOP: invalid candidate input or private output\n')
				expect(fs.existsSync(path.join(parent, 'out'))).toBe(false)
			}
		} finally { fs.rmSync(parent, { recursive: true, force: true }) }
	})
})
