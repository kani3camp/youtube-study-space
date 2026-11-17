import { readFileSync } from 'node:fs'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
	OPERATOR_NAME,
	POLICY_EFFECTIVE_DATE,
	POLICY_VERSIONS,
	publicPolicyConfig,
} from './public-policy-config'

afterEach(() => vi.unstubAllEnvs())

// サーバーが同意を受け付ける版の定数。フロントエンドとずれるとログインを開始できない。
function backendPolicyVersions() {
	const source = readFileSync(
		new URL('../../system/core/mypage/policy.go', import.meta.url),
		'utf8',
	)
	const constant = (name: string) =>
		source.match(new RegExp(`^\\s*${name}\\s*=\\s*"([^"]+)"`, 'm'))?.[1]
	return {
		privacy: constant('CurrentPrivacyPolicyVersion'),
		terms: constant('CurrentTermsVersion'),
	}
}

function calendarDate(value: string) {
	if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
	const [year, month, day] = value.split('-').map(Number)
	const date = new Date(Date.UTC(year, month - 1, day))
	return (
		date.getUTCFullYear() === year &&
		date.getUTCMonth() === month - 1 &&
		date.getUTCDate() === day
	)
}

describe('public policy constants', () => {
	it('uses the same consent versions as the backend', () => {
		expect(backendPolicyVersions()).toEqual(POLICY_VERSIONS)
	})
	it('keeps the effective date unset or a real calendar date', () => {
		if (POLICY_EFFECTIVE_DATE !== undefined)
			expect(calendarDate(POLICY_EFFECTIVE_DATE)).toBe(true)
		expect(calendarDate('2026-02-29')).toBe(false)
		expect(calendarDate('2028-02-29')).toBe(true)
	})
	it.each([
		[
			'test-youtube-study-space',
			'https://docs.google.com/forms/d/e/1FAIpQLScKyaOQSxKIxs1znN72MlrDwHmJoQ94MnEuUK9UKD_fgqHEzg/viewform',
		],
		[
			'youtube-study-space',
			'https://docs.google.com/forms/d/e/1FAIpQLSfTvdRD8r3QPOBMNjNwixzaITe5ca-BPst--ZOoK1aFZXAy2Q/viewform',
		],
	])('selects the fixed shared form for %s', (project, form) => {
		vi.stubEnv('VITE_FIREBASE_PROJECT_ID', project)
		expect(publicPolicyConfig()).toEqual({
			operatorName: OPERATOR_NAME,
			contactURL: form,
			effectiveDate: POLICY_EFFECTIVE_DATE,
		})
	})
	it.each([
		undefined,
		'',
		'synthetic-unknown-project',
	])('shows no contact destination for unknown project %s', (project) => {
		vi.stubEnv('VITE_FIREBASE_PROJECT_ID', project)
		vi.stubEnv('VITE_PUBLIC_CONTACT_URL', 'mailto:retired@example.invalid')
		expect(publicPolicyConfig().contactURL).toBeUndefined()
		expect(publicPolicyConfig().operatorName).toBe('らいど開発')
	})
})
