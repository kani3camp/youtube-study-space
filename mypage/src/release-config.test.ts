import { spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
	checkReleaseConfig,
	checkReleaseConfigJSON,
	type ReleaseConfig,
	type ReleaseConfigReport,
	releaseConfigMaxBytes,
} from './release-config'

function fixture(): ReleaseConfig {
	const deployment = {
		environment: 'development' as const,
		projectID: 'demo-synthetic-mypage',
		projectNumber: '123456789',
		webAppID: '1:123456789:web:synthetic',
		publicOrigin: 'https://example.invalid',
		privacyVersion: 'synthetic-privacy-v1',
		termsVersion: 'synthetic-terms-v1',
	}
	return {
		schemaVersion: 1,
		deployment,
		frontend: {
			VITE_FIREBASE_API_KEY: 'synthetic-public-api-key',
			VITE_FIREBASE_AUTH_DOMAIN: `${deployment.projectID}.firebaseapp.com`,
			VITE_FIREBASE_PROJECT_ID: deployment.projectID,
			VITE_FIREBASE_APP_ID: deployment.webAppID,
			VITE_APP_CHECK_SITE_KEY: 'synthetic-public-site-key',
			VITE_PRIVACY_POLICY_VERSION: deployment.privacyVersion,
			VITE_TERMS_VERSION: deployment.termsVersion,
		},
		backend: { ...deployment },
		hosting: {
			environment: deployment.environment,
			projectID: deployment.projectID,
			publicOrigin: deployment.publicOrigin,
			serviceID: 'synthetic-mypage',
			region: 'asia-southeast2',
			target: 'synthetic-web',
			inventory: {
				reviewed: true,
				script: ['https://scripts.example.invalid/enterprise.js'],
				connect: ['https://api.example.invalid'],
				frame: ['https://frames.example.invalid'],
				image: ['https://images.example.invalid'],
				analyticsApproved: false,
			},
		},
	}
}
function diagnostic(result: ReleaseConfigReport, field: string, code: string) {
	expect(result.configurationValid).toBe(false)
	expect(result.diagnostics).toEqual(
		expect.arrayContaining([expect.objectContaining({ field, code })]),
	)
}
function cli(args: string[], input?: string | Uint8Array) {
	return spawnSync(
		process.execPath,
		[
			fileURLToPath(
				new URL('../scripts/check-release-config.mjs', import.meta.url),
			),
			...args,
		],
		{
			input,
			encoding: 'utf8',
			timeout: 10_000,
			env: {
				...process.env,
				VITE_FIREBASE_API_KEY: 'AMBIENT_SECRET_SENTINEL',
				GOOGLE_APPLICATION_CREDENTIALS: '/private/AMBIENT_CREDENTIAL_SENTINEL',
				FIRESTORE_EMULATOR_HOST: 'external.example.invalid:8080',
			},
		},
	)
}

describe('offline public release configuration', () => {
	it('validates development and production pairs without claiming readiness or echoing values', () => {
		const input = fixture()
		const development = checkReleaseConfig(input)
		expect(development.configurationValid).toBe(true)
		expect(development.checks.backendCompanion).toBe('consistent')
		expect(development.checks.hostingCandidate).toBe('consistent')
		input.deployment.environment = 'production'
		input.backend = { ...input.deployment }
		if (input.hosting) {
			input.hosting.environment = 'production'
			input.hosting.region = 'asia-northeast2'
		}
		const production = checkReleaseConfigJSON(JSON.stringify(input))
		expect(production.configurationValid).toBe(true)
		for (const result of [development, production]) {
			expect(result.releaseReady).toBe(false)
			expect(result.pendingGates).toContain(
				'project-app-auth-domain-registration',
			)
			expect(result.pendingGates).toContain(
				'real-hosting-oauth-app-check-auth-e2e',
			)
			expect(result.pendingGates).toContain(
				'D01-D02-D03-owner-decisions-and-publication-review',
			)
		}
		for (const value of Object.values(input.deployment))
			expect(JSON.stringify(production)).not.toContain(value)
	})
	it('keeps absent backend and Hosting companions explicitly unverified', () => {
		const input = fixture()
		delete input.backend
		delete input.hosting
		const result = checkReleaseConfig(input)
		expect(result.configurationValid).toBe(true)
		expect(result.checks.backendCompanion).toBe('not-supplied')
		expect(result.checks.hostingCandidate).toBe('not-supplied')
		expect(result.checks.publicPolicy).toBe('not-supplied')
		expect(result.releaseReady).toBe(false)
	})
	it('reuses safe publication validation while keeping optional operator identity and unresolved decisions pending', () => {
		const input = fixture()
		input.publicPolicy = {
			VITE_PUBLIC_CONTACT_URL: 'https://general-contact.example.invalid',
			VITE_PUBLIC_PRIVACY_CONTACT_URL: 'mailto:privacy@example.invalid',
			VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: '2026-10-07',
			VITE_PUBLIC_TERMS_EFFECTIVE_DATE: '2026-10-07',
			VITE_PUBLIC_JURISDICTION: 'Synthetic public jurisdiction',
		}
		const result = checkReleaseConfig(input)
		expect(result.configurationValid).toBe(true)
		expect(result.checks.publicPolicy).toBe('consistent')
		expect(result.releaseReady).toBe(false)
		expect(result.pendingGates).toContain(
			'D01-D02-D03-owner-decisions-and-publication-review',
		)
		for (const value of Object.values(input.publicPolicy))
			expect(JSON.stringify(result)).not.toContain(value)
		delete input.publicPolicy.VITE_PUBLIC_PRIVACY_CONTACT_URL
		diagnostic(
			checkReleaseConfig(input),
			'publicPolicy.VITE_PUBLIC_PRIVACY_CONTACT_URL',
			'MISSING_FIELD',
		)
		input.publicPolicy.VITE_PUBLIC_PRIVACY_CONTACT_URL =
			'mailto:private@example.invalid?body=PRIVATE_VALUE_SENTINEL'
		diagnostic(
			checkReleaseConfig(input),
			'publicPolicy.VITE_PUBLIC_PRIVACY_CONTACT_URL',
			'INVALID_FIELD',
		)
		const forbidden = checkReleaseConfig({
			...input,
			publicPolicy: {
				...input.publicPolicy,
				PRIVATE_KEY_SENTINEL: 'PRIVATE_VALUE_SENTINEL',
			},
		})
		diagnostic(forbidden, 'publicPolicy', 'UNEXPECTED_FIELD')
		expect(JSON.stringify(forbidden)).not.toContain('SENTINEL')
	})
	it('reports missing and malformed fields using known field paths', () => {
		const input = fixture()
		delete input.frontend.VITE_FIREBASE_API_KEY
		input.frontend.VITE_APP_CHECK_SITE_KEY = ''
		input.frontend.VITE_FIREBASE_AUTH_DOMAIN =
			'https://private:secret@example.invalid'
		input.deployment.projectNumber = '0123456789'
		input.deployment.privacyVersion = 'contains whitespace'
		const result = checkReleaseConfig(input)
		for (const [field, code] of [
			['frontend.VITE_FIREBASE_API_KEY', 'MISSING_FIELD'],
			['frontend.VITE_APP_CHECK_SITE_KEY', 'MISSING_FIELD'],
			['frontend.VITE_FIREBASE_AUTH_DOMAIN', 'INVALID_FIELD'],
			['deployment.projectNumber', 'INVALID_FIELD'],
			['deployment.privacyVersion', 'INVALID_FIELD'],
		])
			diagnostic(result, field, code)
		diagnostic(checkReleaseConfig({}), 'deployment', 'MISSING_FIELD')
		diagnostic(checkReleaseConfig([]), 'input', 'INVALID_FIELD')
		diagnostic(
			checkReleaseConfig({ ...fixture(), schemaVersion: 2 }),
			'schemaVersion',
			'INVALID_FIELD',
		)
	})
	it.each([
		['VITE_FIREBASE_PROJECT_ID', 'other-synthetic-project'],
		['VITE_FIREBASE_APP_ID', '1:999999999:web:other'],
		['VITE_PRIVACY_POLICY_VERSION', 'other-privacy-v2'],
		['VITE_TERMS_VERSION', 'other-terms-v2'],
		['VITE_FIREBASE_AUTH_DOMAIN', 'other-synthetic-project.firebaseapp.com'],
	] as const)('rejects mixed frontend %s', (key, value) => {
		const input = fixture()
		input.frontend[key] = value
		diagnostic(checkReleaseConfig(input), `frontend.${key}`, 'MISMATCH')
	})
	it('rejects app/project-number mismatch and leaves custom-domain registration pending', () => {
		const input = fixture()
		input.deployment.webAppID = '1:987654321:web:synthetic'
		diagnostic(checkReleaseConfig(input), 'deployment.webAppID', 'MISMATCH')
		const custom = fixture()
		custom.frontend.VITE_FIREBASE_AUTH_DOMAIN = 'auth.example.invalid'
		expect(checkReleaseConfig(custom).configurationValid).toBe(true)
		expect(checkReleaseConfig(custom).pendingGates).toContain(
			'project-app-auth-domain-registration',
		)
	})
	it.each([
		'environment',
		'projectID',
		'projectNumber',
		'webAppID',
		'publicOrigin',
		'privacyVersion',
		'termsVersion',
	] as const)('rejects mixed backend %s', (key) => {
		const input = fixture()
		if (!input.backend) throw new Error('Missing synthetic companion')
		if (key === 'environment') input.backend.environment = 'production'
		else if (key === 'publicOrigin')
			input.backend.publicOrigin = 'https://other.invalid'
		else input.backend[key] = 'other-synthetic-value'
		const result = checkReleaseConfig(input)
		diagnostic(result, `backend.${key}`, 'MISMATCH')
		expect(result.checks.backendCompanion).toBe('invalid')
	})
	it.each([
		'http://example.invalid',
		'https://user:private@example.invalid',
		'https://example.invalid/',
		'https://example.invalid/login',
		'https://example.invalid?code=private',
		'https://example.invalid#private',
		'https://example.invalid:8443',
	])('rejects noncanonical origin %s', (value) => {
		const input = fixture()
		input.deployment.publicOrigin = value
		diagnostic(
			checkReleaseConfig(input),
			'deployment.publicOrigin',
			'INVALID_FIELD',
		)
	})
	it('requires analytics ID/origin alignment and separate candidate inventory approval', () => {
		const input = fixture()
		input.frontend.VITE_MYPAGE_ANALYTICS_READY = 'true'
		const missing = checkReleaseConfig(input)
		diagnostic(missing, 'frontend.VITE_MYPAGE_GA4_ID', 'MISSING_FIELD')
		diagnostic(missing, 'frontend.VITE_MYPAGE_PUBLIC_ORIGIN', 'MISSING_FIELD')
		diagnostic(missing, 'hosting.inventory.analyticsApproved', 'MISMATCH')
		input.frontend.VITE_MYPAGE_GA4_ID = 'G-SYNTHETIC1'
		input.frontend.VITE_MYPAGE_PUBLIC_ORIGIN = 'https://other.invalid'
		diagnostic(
			checkReleaseConfig(input),
			'frontend.VITE_MYPAGE_PUBLIC_ORIGIN',
			'MISMATCH',
		)
		input.frontend.VITE_MYPAGE_PUBLIC_ORIGIN = input.deployment.publicOrigin
		if (input.hosting) input.hosting.inventory.analyticsApproved = true
		diagnostic(
			checkReleaseConfig(input),
			'hosting.inventory.script',
			'MISMATCH',
		)
		input.hosting?.inventory.script.push(
			'https://www.googletagmanager.com/gtag/js',
		)
		expect(checkReleaseConfig(input).configurationValid).toBe(true)
		expect(checkReleaseConfig(input).releaseReady).toBe(false)
		input.frontend.VITE_MYPAGE_ANALYTICS_READY = 'TRUE'
		diagnostic(
			checkReleaseConfig(input),
			'frontend.VITE_MYPAGE_ANALYTICS_READY',
			'INVALID_FIELD',
		)
		input.frontend.VITE_MYPAGE_ANALYTICS_READY = 'false'
		input.frontend.VITE_MYPAGE_GA4_ID = 'invalid'
		diagnostic(
			checkReleaseConfig(input),
			'frontend.VITE_MYPAGE_GA4_ID',
			'INVALID_FIELD',
		)
	})
	it('reuses Hosting region/source/review rejection without returning a deployment candidate', () => {
		const input = fixture()
		if (!input.hosting) throw new Error('Missing synthetic hosting')
		input.hosting.region = 'asia-northeast2'
		diagnostic(checkReleaseConfig(input), 'hosting', 'INVALID_FIELD')
		input.hosting.region = 'asia-southeast2'
		input.hosting.environment = 'production'
		diagnostic(checkReleaseConfig(input), 'hosting.environment', 'MISMATCH')
		input.hosting.environment = 'development'
		input.hosting.inventory.reviewed = false
		diagnostic(checkReleaseConfig(input), 'hosting', 'INVALID_FIELD')
		input.hosting.inventory.reviewed = true
		input.hosting.inventory.script.push(
			'https://www.googletagmanager.com/gtag/js',
		)
		diagnostic(checkReleaseConfig(input), 'hosting', 'INVALID_FIELD')
		input.hosting.inventory.analyticsApproved = true
		expect(checkReleaseConfig(input).configurationValid).toBe(true)
		input.hosting.inventory.connect = [
			'https://private:SECRET_SENTINEL@api.example.invalid',
		]
		const result = checkReleaseConfig(input)
		diagnostic(result, 'hosting', 'INVALID_FIELD')
		expect(JSON.stringify(result)).not.toContain('SECRET_SENTINEL')
		expect(result).not.toHaveProperty('firebase')
	})
	it.each([
		'VITE_OAUTH_CLIENT_SECRET',
		'VITE_YOUTUBE_API_KEY',
		'VITE_FIREBASE_ADMIN_CREDENTIAL',
		'FIREBASE_APPCHECK_DEBUG_TOKEN',
		'GOOGLE_APPLICATION_CREDENTIALS',
		'PRIVATE_KEY_SENTINEL',
	])('rejects forbidden public field %s without echoing keys/values', (key) => {
		const input = fixture()
		const result = checkReleaseConfig({
			...input,
			frontend: { ...input.frontend, [key]: 'PRIVATE_VALUE_SENTINEL' },
		})
		diagnostic(result, 'frontend', 'UNEXPECTED_FIELD')
		expect(JSON.stringify(result)).not.toContain(key)
		expect(JSON.stringify(result)).not.toContain('PRIVATE_VALUE_SENTINEL')
	})
	it('rejects secret fields and fabricated readiness evidence throughout the manifest', () => {
		const input = fixture()
		for (const [value, field] of [
			[{ ...input, infrastructureReady: true }, 'input'],
			[
				{
					...input,
					deployment: { ...input.deployment, credential: 'private' },
				},
				'deployment',
			],
			[
				{ ...input, backend: { ...input.backend, oauthSecret: 'private' } },
				'backend',
			],
			[
				{ ...input, hosting: { ...input.hosting, credential: 'private' } },
				'hosting',
			],
			[
				{
					...input,
					hosting: {
						...input.hosting,
						inventory: { ...input.hosting?.inventory, debugToken: 'private' },
					},
				},
				'hosting.inventory',
			],
		] as const)
			diagnostic(checkReleaseConfig(value), field, 'UNEXPECTED_FIELD')
	})
	it('rejects malformed, duplicate, escaped-duplicate, overnested and oversized JSON safely', () => {
		for (const source of [
			'{"secret":"PRIVATE_SENTINEL",',
			'{"schemaVersion":1,"schemaVersion":2}',
			'{"schemaVersion":1,"\\u0073chemaVersion":1}',
			'{"frontend":{"VITE_FIREBASE_API_KEY":"first","VITE_FIREBASE_API_KEY":"PRIVATE_SENTINEL"}}',
			`${'['.repeat(20)}0${']'.repeat(20)}`,
		]) {
			const result = checkReleaseConfigJSON(source)
			diagnostic(result, 'input', 'INVALID_JSON')
			expect(JSON.stringify(result)).not.toContain('PRIVATE_SENTINEL')
		}
		diagnostic(
			checkReleaseConfigJSON(' '.repeat(releaseConfigMaxBytes + 1)),
			'input',
			'INPUT_TOO_LARGE',
		)
		diagnostic(
			checkReleaseConfigJSON('あ'.repeat(releaseConfigMaxBytes / 2)),
			'input',
			'INPUT_TOO_LARGE',
		)
		const validJSON = checkReleaseConfigJSON(
			JSON.stringify({ arbitrary: [{ 'quote"': 'a,}\\"' }] }),
		)
		expect(validJSON.diagnostics.map((x) => x.code)).not.toContain(
			'INVALID_JSON',
		)
	})
})

describe('explicit bounded configuration CLI', () => {
	it('accepts stdin/selected file, ignores ambient configuration and emits only sanitized verdicts', () => {
		const directory = mkdtempSync(join(tmpdir(), 'mypage-release-config-'))
		try {
			const path = join(directory, 'PRIVATE_FILENAME_SENTINEL.json')
			writeFileSync(path, JSON.stringify(fixture()))
			for (const result of [
				cli(['--stdin'], JSON.stringify(fixture())),
				cli(['--file', path]),
			]) {
				expect(result.status).toBe(0)
				expect(result.stderr).toBe('')
				const report: ReleaseConfigReport = JSON.parse(result.stdout)
				expect(report.configurationValid).toBe(true)
				expect(report.releaseReady).toBe(false)
				expect(result.stdout).not.toMatch(
					/SENTINEL|synthetic-public-api-key|123456789|example\.invalid/,
				)
			}
		} finally {
			rmSync(directory, { recursive: true, force: true })
		}
	})
	it('rejects implicit input, filesystem errors, unsupported options and private malformed JSON without disclosure', () => {
		for (const [args, input] of [
			[[], JSON.stringify(fixture())],
			[['--file', '/missing/PRIVATE_PATH_SENTINEL.json'], undefined],
			[['--file', tmpdir()], undefined],
			[['--stdin', '--secret', 'PRIVATE_ARG_SENTINEL'], '{}'],
			[['--stdin'], '{"PRIVATE_KEY_SENTINEL":"PRIVATE_VALUE_SENTINEL",'],
			[
				['--stdin'],
				JSON.stringify({ ...fixture(), secret: 'PRIVATE_VALUE_SENTINEL' }),
			],
		] as const) {
			const result = cli([...args], input)
			expect(result.status).toBe(1)
			expect(result.stderr).toBe('')
			expect(result.stdout).not.toContain('SENTINEL')
			expect(JSON.parse(result.stdout).releaseReady).toBe(false)
		}
	})
	it('enforces file/stdin byte bounds and rejects malformed UTF-8', () => {
		const directory = mkdtempSync(join(tmpdir(), 'mypage-release-config-'))
		try {
			const path = join(directory, 'oversized.json')
			const input = ' '.repeat(releaseConfigMaxBytes + 1)
			writeFileSync(path, input)
			for (const result of [cli(['--file', path]), cli(['--stdin'], input)]) {
				expect(result.status).toBe(1)
				diagnostic(JSON.parse(result.stdout), 'input', 'INPUT_TOO_LARGE')
			}
			const result = cli(['--stdin'], Uint8Array.from([0xff, 0xfe]))
			expect(result.status).toBe(1)
			diagnostic(JSON.parse(result.stdout), 'input', 'INVALID_JSON')
		} finally {
			rmSync(directory, { recursive: true, force: true })
		}
	})
})
