import * as fs from 'node:fs'
import * as path from 'node:path'
import { createHash } from 'node:crypto'
import { isDeepStrictEqual } from 'node:util'
import { cloud_assembly_schema as schema, cx_api } from 'aws-cdk-lib'

type Json = Record<string, any>
type MutableArtifact = { -readonly [K in keyof schema.ArtifactManifest]: schema.ArtifactManifest[K] }

// This preparer is deliberately limited to the existing development stack.
const ACCOUNT = '657533259235'
const REGION = 'ap-northeast-1'
const STACK = 'AwsCdkStack'

export interface BatchImageCandidateInput {
	baselineTemplate: Json
	stackArtifact: schema.ArtifactManifest
	stackReceipt: {
		StackId: string
		StackStatus: string
		RoleARN: string
		EnableTerminationProtection: boolean
	}
	taskLogicalId: string
	candidateImage: string
	previousImage: string
}

function requireValue(condition: unknown, reason: string): asserts condition {
	if (!condition) throw new Error(reason)
}
const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))
const jsonText = (value: unknown) => JSON.stringify(value, null, 2) + '\n'
const hash = (value: unknown) => createHash('sha256').update(jsonText(value)).digest('hex')

// Resolve only pseudo parameters whose values are fixed by this dev target.
const substitute = (value: string): string => value
	.replace(/\$\{AWS::AccountId\}/g, ACCOUNT)
	.replace(/\$\{AWS::Region\}/g, REGION)
	.replace(/\$\{AWS::Partition\}/g, 'aws')
	.replace(/\$\{AWS::URLSuffix\}/g, 'amazonaws.com')

const imageString = (value: any): string => {
	if (typeof value === 'string') return value
	if (value && typeof value['Fn::Sub'] === 'string') {
		const result = substitute(value['Fn::Sub'])
		requireValue(!result.includes('${'), 'unresolved baseline image')
		return result
	}
	throw new Error('unsupported baseline image')
}

const renderDefinition = (value: any): string => {
	if (typeof value === 'string') return value
	if (value && Array.isArray(value['Fn::Join'])) {
		const [separator, parts] = value['Fn::Join']
		requireValue(typeof separator === 'string' && Array.isArray(parts), 'unsupported ASL join')
		return parts.map((part: any) => renderDefinition(part)).join(separator)
	}
	// Intrinsics in source CDK ASL are quoted strings. They cannot be task
	// references for this bounded candidate: those must be the literal family.
	if (value?.Ref === 'AWS::Partition') return 'aws'
	if (value?.Ref || value?.['Fn::GetAtt']) return '__UNRESOLVED_INTRINSIC__'
	throw new Error('unsupported state machine definition')
}

const familyConsumers = (template: Json, family: string) => {
	const consumers: Array<{ stateMachineLogicalId: string; statePath: string; job: string }> = []
	for (const [id, resource] of Object.entries(template.Resources) as Array<[string, Json]>) {
		if (resource.Type !== 'AWS::StepFunctions::StateMachine') continue
		const props = resource.Properties
		const definition = props?.Definition ?? JSON.parse(renderDefinition(props?.DefinitionString))
		const visit = (value: any, statePath: string) => {
			if (!value || typeof value !== 'object') return
			if (value.Type === 'Task') requireValue(typeof value.Resource === 'string' &&
				!value.Resource.includes('__UNRESOLVED_INTRINSIC__'), 'unmapped task resource')
			if (value.Type === 'Task' && typeof value.Resource === 'string' &&
				value.Resource.includes(':states:::ecs:runTask')) {
				requireValue(value.Parameters?.TaskDefinition === family, 'unmapped ECS task reference')
				const containers = value.Parameters?.Overrides?.ContainerOverrides
				requireValue(Array.isArray(containers), 'unmapped ECS job override')
				const batch = containers.filter((c: Json) => c.Name === 'daily-batch')
				const jobs = batch[0]?.Environment?.filter((e: Json) => e.Name === 'JOB')
				requireValue(batch.length === 1 && jobs?.length === 1 && typeof jobs[0].Value === 'string',
					'unmapped ECS job override')
				consumers.push({ stateMachineLogicalId: id, statePath, job: jobs[0].Value })
			}
			for (const [key, child] of Object.entries(value)) visit(child, `${statePath}/${key}`)
		}
		visit(definition, '')
	}
	requireValue(consumers.length > 0, 'no shared-family consumers')
	return consumers
}

export function prepareBatchImageCandidate(input: BatchImageCandidateInput) {
	const receipt = input.stackReceipt
	requireValue(receipt?.StackId?.startsWith(`arn:aws:cloudformation:${REGION}:${ACCOUNT}:stack/${STACK}/`),
		'dev stack receipt required')
	requireValue(['CREATE_COMPLETE', 'UPDATE_COMPLETE', 'UPDATE_ROLLBACK_COMPLETE'].includes(receipt.StackStatus),
		'stable stack receipt required')
	requireValue(typeof receipt.EnableTerminationProtection === 'boolean', 'termination protection receipt required')
	const artifact: MutableArtifact = clone(input.stackArtifact)
	requireValue(artifact?.type === schema.ArtifactType.AWS_CLOUDFORMATION_STACK, 'CDK stack artifact required')
	requireValue(artifact.environment === `aws://${ACCOUNT}/${REGION}` ||
		artifact.environment === 'aws://unknown-account/unknown-region', 'unexpected CDK environment')
	const properties = artifact.properties as Json
	requireValue(properties && (!properties.stackName || properties.stackName === STACK), 'unexpected stack name')
	requireValue(!properties.parameters || Object.keys(properties.parameters).length === 0, 'use existing parameters')
	const executionRole = typeof properties.cloudFormationExecutionRoleArn === 'string'
		? substitute(properties.cloudFormationExecutionRoleArn) : undefined
	requireValue(executionRole === receipt.RoleARN &&
		receipt.RoleARN?.startsWith(`arn:aws:iam::${ACCOUNT}:role/`), 'existing execution role mismatch')
	requireValue(typeof properties.assumeRoleArn === 'string' &&
		substitute(properties.assumeRoleArn).startsWith(`arn:aws:iam::${ACCOUNT}:role/`), 'existing CDK deploy role required')

	const baseline = clone(input.baselineTemplate)
	requireValue(baseline && !baseline.Transform && baseline.Resources, 'plain CloudFormation template required')
	const task = baseline.Resources[input.taskLogicalId]
	requireValue(task?.Type === 'AWS::ECS::TaskDefinition' && !task.Condition, 'existing task logical ID required')
	const props = task.Properties
	requireValue(typeof props?.Family === 'string' && props.Family.length > 0, 'explicit existing family required')
	requireValue(props.RuntimePlatform?.CpuArchitecture === 'ARM64' &&
		props.RuntimePlatform?.OperatingSystemFamily === 'LINUX' && props.NetworkMode === 'awsvpc' &&
		props.Cpu === '256' && props.Memory === '512', 'unexpected batch runtime')
	const containers = props.ContainerDefinitions
	requireValue(Array.isArray(containers) && containers.length === 1 && containers[0].Name === 'daily-batch',
		'existing single batch container required')
	const originalImage = containers[0].Image
	const baselineImage = imageString(originalImage)
	const ecr = new RegExp(`^${ACCOUNT}\\.dkr\\.ecr\\.${REGION}\\.amazonaws\\.com/([a-z0-9][a-z0-9._/-]*)(?::[^/@]+|@sha256:[a-f0-9]{64})$`)
	const match = baselineImage.match(ecr)
	requireValue(match, 'existing dev ECR image required')
	const repository = `${ACCOUNT}.dkr.ecr.${REGION}.amazonaws.com/${match[1]}`
	const immutable = new RegExp(`^${repository.split('.').join('\\.')}@sha256:[a-f0-9]{64}$`)
	requireValue(immutable.test(input.candidateImage),
		'same-repository immutable candidate required')
	requireValue(immutable.test(input.previousImage), 'same-repository immutable rollback required')
	if (baselineImage.includes('@sha256:')) requireValue(baselineImage === input.previousImage, 'baseline digest mismatch')
	requireValue(input.candidateImage !== input.previousImage, 'candidate must change image')
	const consumers = familyConsumers(baseline, props.Family)
	const candidate = clone(baseline)
	candidate.Resources[input.taskLogicalId].Properties.ContainerDefinitions[0].Image = input.candidateImage
	const rollback = clone(baseline)
	rollback.Resources[input.taskLogicalId].Properties.ContainerDefinitions[0].Image = input.previousImage
	const restored = clone(candidate)
	restored.Resources[input.taskLogicalId].Properties.ContainerDefinitions[0].Image = clone(originalImage)
	requireValue(isDeepStrictEqual(restored, baseline), 'non-image template change')

	// Keep the existing CDK roles/bootstrap requirements. Do not carry asset
	// publication or the old template S3 URL into this pre-published-image assembly.
	delete properties.stackTemplateAssetObjectUrl
	delete properties.additionalDependencies
	delete artifact.dependencies
	delete artifact.metadata
	delete artifact.additionalMetadataFile
	properties.templateFile = `${STACK}.template.json`
	properties.stackName = STACK
	properties.terminationProtection = receipt.EnableTerminationProtection
	artifact.environment = `aws://${ACCOUNT}/${REGION}`
	return {
		candidate, rollback, original: baseline, artifact,
		review: {
			status: 'OFFLINE_CANDIDATE_LIVE_PROVENANCE_AND_APPROVAL_REQUIRED',
			stackId: receipt.StackId,
			changedPath: `/Resources/${input.taskLogicalId}/Properties/ContainerDefinitions/0/Image`,
			before: originalImage, after: input.candidateImage,
			rollbackImage: input.previousImage,
			family: props.Family, templateConsumers: consumers,
			baselineSha256: hash(baseline), candidateSha256: hash(candidate), rollbackSha256: hash(rollback),
			assetPublicationCount: 0,
			registrationEffect: 'TaskDefinition replacement registers an ACTIVE revision. Every unqualified family reference may resolve it.',
			rollbackEffect: 'Applying rollback registers another revision with the recorded previous digest; it does not restore the original revision number. The original template is retained separately.',
		},
	}
}

export function writeBatchImageCandidate(input: BatchImageCandidateInput, outdir: string) {
	const prepared = prepareBatchImageCandidate(input)
	// Fresh output only. Owner-only files; no overwrite, cloud client or subprocess.
	fs.mkdirSync(outdir, { mode: 0o700 })
	fs.writeFileSync(path.join(outdir, 'original.template.json'), jsonText(prepared.original), { mode: 0o600, flag: 'wx' })
	for (const [phase, template] of [['candidate', prepared.candidate], ['rollback', prepared.rollback]] as const) {
		const dir = path.join(outdir, phase)
		fs.mkdirSync(dir, { mode: 0o700 })
		fs.writeFileSync(path.join(dir, `${STACK}.template.json`), jsonText(template), { mode: 0o600, flag: 'wx' })
		const builder = new cx_api.CloudAssemblyBuilder(dir)
		builder.addArtifact(STACK, prepared.artifact)
		builder.buildAssembly()
		fs.chmodSync(path.join(dir, 'manifest.json'), 0o600)
	}
	fs.writeFileSync(path.join(outdir, 'review.json'), jsonText(prepared.review), { mode: 0o600, flag: 'wx' })
	return prepared.review
}
