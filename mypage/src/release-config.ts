import { type HostingInputs, hostingCandidate } from './hosting.ts'
import {
	type PublicPolicyField,
	validatePublicPolicyConfig,
} from './public-policy-config.ts'

// Offline public configuration only. This module does not import Firebase,
// initialize a browser runtime, read the environment or claim release readiness.
export const releaseConfigMaxBytes = 65_536

const deploymentFields = [
	'environment',
	'projectID',
	'projectNumber',
	'webAppID',
	'publicOrigin',
	'privacyVersion',
	'termsVersion',
] as const
const frontendFields = [
	'VITE_FIREBASE_API_KEY',
	'VITE_FIREBASE_AUTH_DOMAIN',
	'VITE_FIREBASE_PROJECT_ID',
	'VITE_FIREBASE_APP_ID',
	'VITE_APP_CHECK_SITE_KEY',
	'VITE_PRIVACY_POLICY_VERSION',
	'VITE_TERMS_VERSION',
	'VITE_MYPAGE_ANALYTICS_READY',
	'VITE_MYPAGE_GA4_ID',
	'VITE_MYPAGE_PUBLIC_ORIGIN',
	'VITE_PRIVACY_INTAKE_ENABLED',
] as const

type DeploymentField = (typeof deploymentFields)[number]
export type ReleaseDeployment = Record<DeploymentField, string> & {
	environment: 'development' | 'production'
}
export type ReleaseConfig = {
	schemaVersion: 1
	deployment: ReleaseDeployment
	frontend: Partial<Record<(typeof frontendFields)[number], string>>
	// Optional public companion declaration, never a server environment export.
	backend?: ReleaseDeployment
	hosting?: HostingInputs
	publicPolicy?: Partial<Record<PublicPolicyField, string>>
}
export type ReleaseDiagnostic = {
	field: string
	code:
		| 'MISSING_FIELD'
		| 'INVALID_FIELD'
		| 'MISMATCH'
		| 'UNEXPECTED_FIELD'
		| 'INVALID_JSON'
		| 'INPUT_TOO_LARGE'
		| 'INPUT_UNAVAILABLE'
	action: string
}
export type ReleaseConfigReport = {
	schemaVersion: 1
	configurationValid: boolean
	releaseReady: false
	checks: {
		backendCompanion: 'not-supplied' | 'invalid' | 'consistent'
		hostingCandidate: 'not-supplied' | 'invalid' | 'consistent'
		publicPolicy: 'not-supplied' | 'invalid' | 'consistent'
	}
	diagnostics: ReleaseDiagnostic[]
	pendingGates: string[]
}

function report(diagnostics: ReleaseDiagnostic[]): ReleaseConfigReport {
	return {
		schemaVersion: 1,
		configurationValid: diagnostics.length === 0,
		releaseReady: false,
		checks: {
			backendCompanion: 'not-supplied',
			hostingCandidate: 'not-supplied',
			publicPolicy: 'not-supplied',
		},
		diagnostics,
		// Supplied readiness/review flags are declarations, never evidence.
		pendingGates: [
			'project-app-auth-domain-registration',
			'infrastructure-ready-gate-and-runtime-iam',
			'real-hosting-oauth-app-check-auth-e2e',
			'real-sdk-csp-and-analytics-network-inventory',
			'analytics-console-consent-and-retention-review',
			'data-inventory-cleanup-monitoring-and-rollback',
			'D01-implementation-D02-policy-compliance-D03-public-values-and-publication-review',
			'privacy-intake-operator-reply-and-no-session-exception-contact',
		],
	}
}

export function releaseInputFailure(
	code: 'INVALID_JSON' | 'INPUT_TOO_LARGE' | 'INPUT_UNAVAILABLE',
): ReleaseConfigReport {
	return report([
		{
			field: 'input',
			code,
			action:
				code === 'INPUT_TOO_LARGE'
					? 'Supply an explicit JSON manifest no larger than 65536 bytes.'
					: 'Supply one readable UTF-8 JSON manifest using --file or --stdin; omit credentials and external readiness evidence.',
		},
	])
}

function object(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value)
}
function identifier(value: unknown, max = 256): value is string {
	return (
		typeof value === 'string' &&
		value.length <= max &&
		/^[A-Za-z0-9._-]+$/.test(value)
	)
}
function origin(value: unknown): value is string {
	if (typeof value !== 'string' || value.length > 256) return false
	try {
		const url = new URL(value)
		return (
			url.protocol === 'https:' &&
			url.origin === value &&
			!url.username &&
			!url.password &&
			!url.port &&
			/^[a-z0-9.-]+$/.test(url.hostname) &&
			!url.hostname.includes('..')
		)
	} catch {
		return false
	}
}

export function checkReleaseConfig(input: unknown): ReleaseConfigReport {
	const diagnostics: ReleaseDiagnostic[] = []
	const add = (
		field: string,
		code: ReleaseDiagnostic['code'],
		action: string,
	) => diagnostics.push({ field, code, action })
	const fields = (
		value: unknown,
		path: string,
		allowed: readonly string[],
	): Record<string, unknown> | null => {
		if (!object(value)) {
			add(
				path,
				value === undefined ? 'MISSING_FIELD' : 'INVALID_FIELD',
				'Supply the explicit public configuration object.',
			)
			return null
		}
		if (Object.keys(value).some((key) => !allowed.includes(key)))
			add(
				path,
				'UNEXPECTED_FIELD',
				'Remove fields outside the public manifest schema; keep credentials, secrets and debug settings server-only.',
			)
		return value
	}
	const required = (
		value: Record<string, unknown>,
		key: string,
		path: string,
		validate: (entry: unknown) => boolean,
		action: string,
	) => {
		if (value[key] === undefined || value[key] === '')
			add(`${path}.${key}`, 'MISSING_FIELD', action)
		else if (!validate(value[key]))
			add(`${path}.${key}`, 'INVALID_FIELD', action)
	}
	const equal = (
		value: unknown,
		expected: unknown,
		path: string,
		action: string,
	) => {
		if (
			typeof value === 'string' &&
			typeof expected === 'string' &&
			value !== expected
		)
			add(path, 'MISMATCH', action)
	}
	const deployment = (
		value: unknown,
		path: string,
	): Record<string, unknown> | null => {
		const config = fields(value, path, deploymentFields)
		if (!config) return null
		required(
			config,
			'environment',
			path,
			(x) => x === 'development' || x === 'production',
			'Select development or production explicitly.',
		)
		required(
			config,
			'projectID',
			path,
			(x) => typeof x === 'string' && /^[a-z][a-z0-9-]{4,28}[a-z0-9]$/.test(x),
			'Supply the selected Firebase project ID.',
		)
		required(
			config,
			'projectNumber',
			path,
			(x) => typeof x === 'string' && /^[1-9][0-9]{0,19}$/.test(x),
			'Supply the selected project number as a decimal string.',
		)
		required(
			config,
			'webAppID',
			path,
			(x) =>
				typeof x === 'string' &&
				x.length <= 256 &&
				/^1:[1-9][0-9]{0,19}:web:[A-Za-z0-9_-]+$/.test(x),
			'Supply the public Firebase Web App ID.',
		)
		if (
			typeof config.webAppID === 'string' &&
			typeof config.projectNumber === 'string' &&
			config.webAppID.split(':')[1] !== config.projectNumber
		)
			add(
				`${path}.webAppID`,
				'MISMATCH',
				'Use the Web App ID belonging to the selected project number; confirm registration separately.',
			)
		required(
			config,
			'publicOrigin',
			path,
			origin,
			'Supply the fixed HTTPS origin without a path, port, credentials, query or fragment.',
		)
		for (const key of ['privacyVersion', 'termsVersion'])
			required(
				config,
				key,
				path,
				(x) => identifier(x, 128),
				'Supply the selected policy version identifier using letters, digits, dot, underscore or hyphen.',
			)
		return config
	}

	const root = fields(input, 'input', [
		'schemaVersion',
		'deployment',
		'frontend',
		'backend',
		'hosting',
		'publicPolicy',
	])
	if (!root) return report(diagnostics)
	if (root.schemaVersion !== 1)
		add(
			'schemaVersion',
			'INVALID_FIELD',
			'Use public manifest schema version 1.',
		)
	const selected = deployment(root.deployment, 'deployment')
	const frontend = fields(root.frontend, 'frontend', frontendFields)
	if (frontend) {
		for (const key of [
			'VITE_FIREBASE_API_KEY',
			'VITE_APP_CHECK_SITE_KEY',
			'VITE_FIREBASE_PROJECT_ID',
			'VITE_PRIVACY_POLICY_VERSION',
			'VITE_TERMS_VERSION',
		])
			required(
				frontend,
				key,
				'frontend',
				identifier,
				'Supply this public browser setting; never substitute a token or server credential.',
			)
		required(
			frontend,
			'VITE_FIREBASE_APP_ID',
			'frontend',
			(x) =>
				typeof x === 'string' &&
				x.length <= 256 &&
				/^1:[1-9][0-9]{0,19}:web:[A-Za-z0-9_-]+$/.test(x),
			'Supply the public Firebase Web App ID matching deployment.webAppID.',
		)
		required(
			frontend,
			'VITE_FIREBASE_AUTH_DOMAIN',
			'frontend',
			(x) =>
				typeof x === 'string' &&
				x.length <= 253 &&
				x.split('.').length > 1 &&
				x
					.split('.')
					.every((label) =>
						/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label),
					),
			'Supply a DNS hostname without URL syntax; confirm the Firebase authorized domain separately.',
		)
		if (selected) {
			for (const [key, companion] of [
				['VITE_FIREBASE_PROJECT_ID', 'projectID'],
				['VITE_FIREBASE_APP_ID', 'webAppID'],
				['VITE_PRIVACY_POLICY_VERSION', 'privacyVersion'],
				['VITE_TERMS_VERSION', 'termsVersion'],
			] as const)
				equal(
					frontend[key],
					selected[companion],
					`frontend.${key}`,
					`Match deployment.${companion} for this release; do not mix environment configurations.`,
				)
			const authDomain = frontend.VITE_FIREBASE_AUTH_DOMAIN
			if (
				typeof authDomain === 'string' &&
				/\.(firebaseapp\.com|web\.app)$/.test(authDomain) &&
				authDomain !== `${selected.projectID}.firebaseapp.com` &&
				authDomain !== `${selected.projectID}.web.app`
			)
				add(
					'frontend.VITE_FIREBASE_AUTH_DOMAIN',
					'MISMATCH',
					'Use the default auth domain of the selected project or a separately verified custom auth domain.',
				)
		}
		const analytics = frontend.VITE_MYPAGE_ANALYTICS_READY
		if (
			frontend.VITE_PRIVACY_INTAKE_ENABLED !== undefined &&
			frontend.VITE_PRIVACY_INTAKE_ENABLED !== 'false'
		)
			add(
				'frontend.VITE_PRIVACY_INTAKE_ENABLED',
				'INVALID_FIELD',
				'Keep privacy intake disabled until operator reply, no-session exception contact, D02 policy compliance, D03 public values and public release gates are completed.',
			)
		if (
			analytics !== undefined &&
			analytics !== 'true' &&
			analytics !== 'false'
		)
			add(
				'frontend.VITE_MYPAGE_ANALYTICS_READY',
				'INVALID_FIELD',
				'Use the string true or false; omit the setting to keep analytics transport disabled.',
			)
		if (analytics === 'true' || frontend.VITE_MYPAGE_GA4_ID !== undefined)
			required(
				frontend,
				'VITE_MYPAGE_GA4_ID',
				'frontend',
				(x) => typeof x === 'string' && /^G-[A-Z0-9]{6,20}$/.test(x),
				'Supply a public GA4 measurement ID before explicitly enabling analytics.',
			)
		if (
			analytics === 'true' ||
			frontend.VITE_MYPAGE_PUBLIC_ORIGIN !== undefined
		) {
			required(
				frontend,
				'VITE_MYPAGE_PUBLIC_ORIGIN',
				'frontend',
				origin,
				'Supply the fixed HTTPS public origin before explicitly enabling analytics.',
			)
			if (selected)
				equal(
					frontend.VITE_MYPAGE_PUBLIC_ORIGIN,
					selected.publicOrigin,
					'frontend.VITE_MYPAGE_PUBLIC_ORIGIN',
					'Match deployment.publicOrigin and the final browser origin.',
				)
		}
	}
	const result = report(diagnostics)
	if (root.publicPolicy !== undefined) {
		const policy = validatePublicPolicyConfig(root.publicPolicy)
		for (const issue of policy.issues) {
			const field =
				issue.field === 'input' ? 'publicPolicy' : `publicPolicy.${issue.field}`
			add(
				field,
				issue.code === 'missing'
					? 'MISSING_FIELD'
					: issue.code === 'unknown-field'
						? 'UNEXPECTED_FIELD'
						: 'INVALID_FIELD',
				issue.code === 'unknown-field'
					? 'Supply only the explicit public publication fields; keep secrets and private operator information outside this manifest.'
					: 'Supply the selected public publication value: safe text, an HTTPS/single-recipient mailto contact without query or fragment, or a real YYYY-MM-DD date. The operator display label is optional; publication review remains pending.',
			)
		}
		result.checks.publicPolicy =
			policy.issues.length === 0 ? 'consistent' : 'invalid'
	}
	if (root.backend !== undefined) {
		const start = diagnostics.length
		const backend = deployment(root.backend, 'backend')
		if (selected && backend)
			for (const key of deploymentFields)
				equal(
					backend[key],
					selected[key],
					`backend.${key}`,
					`Match deployment.${key}; supply nonsecret companions only, never export the server environment.`,
				)
		result.checks.backendCompanion =
			diagnostics.length === start ? 'consistent' : 'invalid'
	}
	if (root.hosting !== undefined) {
		const start = diagnostics.length
		const hosting = fields(root.hosting, 'hosting', [
			'environment',
			'projectID',
			'publicOrigin',
			'serviceID',
			'region',
			'target',
			'inventory',
		])
		if (hosting) {
			for (const key of [
				'environment',
				'projectID',
				'publicOrigin',
				'serviceID',
				'region',
				'target',
			])
				required(
					hosting,
					key,
					'hosting',
					(x) => typeof x === 'string' && x.length <= 256,
					'Supply the explicit Hosting candidate setting; existing Hosting validation checks its constraints.',
				)
			const inventory = fields(hosting.inventory, 'hosting.inventory', [
				'reviewed',
				'script',
				'connect',
				'frame',
				'image',
				'analyticsApproved',
			])
			if (inventory) {
				required(
					inventory,
					'reviewed',
					'hosting.inventory',
					(x) => x === true,
					'Supply a reviewed inventory declaration; actual SDK network review remains a separate gate.',
				)
				for (const key of ['script', 'connect', 'frame', 'image'])
					required(
						inventory,
						key,
						'hosting.inventory',
						(x) => Array.isArray(x) && x.length > 0,
						'Supply the reviewed nonempty HTTPS source list; existing Hosting validation rejects unsafe sources.',
					)
			}
			if (selected)
				for (const key of ['environment', 'projectID', 'publicOrigin'])
					equal(
						hosting[key],
						selected[key],
						`hosting.${key}`,
						`Match deployment.${key} for the candidate.`,
					)
			if (inventory && typeof inventory.analyticsApproved !== 'boolean')
				add(
					'hosting.inventory.analyticsApproved',
					'INVALID_FIELD',
					'Supply an explicit boolean for the separate analytics inventory review declaration.',
				)
			if (
				frontend?.VITE_MYPAGE_ANALYTICS_READY === 'true' &&
				inventory?.analyticsApproved !== true
			)
				add(
					'hosting.inventory.analyticsApproved',
					'MISMATCH',
					'An enabled analytics candidate requires a separate approved analytics inventory declaration; real review remains pending.',
				)
			if (
				frontend?.VITE_MYPAGE_ANALYTICS_READY === 'true' &&
				Array.isArray(inventory?.script) &&
				!inventory.script.some(
					(source) =>
						source === 'https://www.googletagmanager.com' ||
						source === 'https://www.googletagmanager.com/' ||
						source === 'https://www.googletagmanager.com/gtag/js',
				)
			)
				add(
					'hosting.inventory.script',
					'MISMATCH',
					'Include the fixed browserTagLoader gtag.js origin or exact path in the reviewed script inventory when analytics is enabled; real vendor connections remain pending.',
				)
			try {
				// Existing pure validator owns CSP, routing, inventory and region rules.
				hostingCandidate(hosting as HostingInputs)
			} catch {
				add(
					'hosting',
					'INVALID_FIELD',
					'Check environment region, project/service/target IDs, exact HTTPS origin and reviewed nonempty HTTPS source lists; do not use wildcards, unsafe directives or credential/query/fragment URLs.',
				)
			}
		}
		result.checks.hostingCandidate =
			diagnostics.length === start ? 'consistent' : 'invalid'
	}
	result.configurationValid = diagnostics.length === 0
	return result
}

// JSON.parse silently overwrites duplicate fields. Walk the already validated
// syntax to reject them and excessive nesting before accepting a manifest.
function hasUniqueKeys(source: string): boolean {
	let index = 0
	const whitespace = () => {
		while (/\s/.test(source[index] ?? '') && index < source.length) index++
	}
	const string = (): string => {
		const start = index++
		while (index < source.length) {
			const character = source[index++]
			if (character === '\\') index++
			else if (character === '"') return JSON.parse(source.slice(start, index))
		}
		throw new Error()
	}
	const walk = (depth: number): boolean => {
		if (depth > 12) return false
		whitespace()
		const character = source[index]
		if (character === '"') {
			string()
			return true
		}
		if (character === '{' || character === '[') {
			index++
			whitespace()
			const keys = new Set<string>()
			const closing = character === '{' ? '}' : ']'
			while (source[index] !== closing) {
				if (character === '{') {
					const key = string()
					if (keys.has(key)) return false
					keys.add(key)
					whitespace()
					index++ // Colon; JSON.parse already checked syntax.
				}
				if (!walk(depth + 1)) return false
				whitespace()
				if (source[index] !== ',') break
				index++
				whitespace()
			}
			index++
			return true
		}
		while (index < source.length && !/[\s,\]}]/.test(source[index] ?? ''))
			index++
		return true
	}
	return walk(0)
}

export function checkReleaseConfigJSON(source: string): ReleaseConfigReport {
	if (new TextEncoder().encode(source).length > releaseConfigMaxBytes)
		return releaseInputFailure('INPUT_TOO_LARGE')
	try {
		const input: unknown = JSON.parse(source)
		if (!hasUniqueKeys(source)) return releaseInputFailure('INVALID_JSON')
		return checkReleaseConfig(input)
	} catch {
		return releaseInputFailure('INVALID_JSON')
	}
}
