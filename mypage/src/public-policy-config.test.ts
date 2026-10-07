import { afterEach, describe, expect, it, vi } from 'vitest'
import {
	PUBLIC_POLICY_FIELDS,
	readPublicPolicyConfig,
	validatePublicPolicyConfig,
} from './public-policy-config'

afterEach(() => vi.unstubAllEnvs())

const credentialPasswordSentinel = 'SYNTHETIC_PASSWORD_SENTINEL'

// Negative URL cases are constructed at runtime from obviously synthetic parts.
// This preserves userinfo rejection without publishing literal Basic Auth URLs.
function credentialTestURL(hostname: string, withPassword: boolean) {
	const url = new URL(`https://${hostname}`)
	url.username = 'synthetic-test-user'
	if (withPassword) url.password = credentialPasswordSentinel
	return url.href
}

const input = {
	VITE_PUBLIC_OPERATOR_NAME: 'Synthetic Service & Friends',
	VITE_PUBLIC_CONTACT_URL: 'https://contact.example.invalid/general',
	VITE_PUBLIC_PRIVACY_CONTACT_URL: 'mailto:privacy+requests@example.invalid',
	VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: '2026-10-07',
	VITE_PUBLIC_TERMS_EFFECTIVE_DATE: '2026-10-08',
	VITE_PUBLIC_JURISDICTION: '合成テスト裁判所',
}

describe('public policy configuration', () => {
	it('accepts explicit public display values without asserting approval', () => {
		expect(validatePublicPolicyConfig(input)).toEqual({
			config: {
				operatorName: input.VITE_PUBLIC_OPERATOR_NAME,
				contactURL: input.VITE_PUBLIC_CONTACT_URL,
				privacyContactURL: input.VITE_PUBLIC_PRIVACY_CONTACT_URL,
				privacyEffectiveDate: '2026-10-07',
				termsEffectiveDate: '2026-10-08',
				jurisdiction: '合成テスト裁判所',
			},
			issues: [],
		})
	})
	it('leaves absent values unset and never requires a public real name', () => {
		const result = validatePublicPolicyConfig({})
		expect(result.config).toEqual({})
		expect(result.issues).toEqual(
			PUBLIC_POLICY_FIELDS.filter((x) => x !== 'VITE_PUBLIC_OPERATOR_NAME').map(
				(field) => ({ field, code: 'missing' }),
			),
		)
		const { VITE_PUBLIC_OPERATOR_NAME: _label, ...withoutLabel } = input
		expect(validatePublicPolicyConfig(withoutLabel).issues).toEqual([])
		expect(
			validatePublicPolicyConfig(withoutLabel).config.operatorName,
		).toBeUndefined()
	})
	it('never promotes the general contact destination to a privacy request channel', () => {
		const result = validatePublicPolicyConfig({
			VITE_PUBLIC_CONTACT_URL: input.VITE_PUBLIC_CONTACT_URL,
		})
		expect(result.config.contactURL).toBe(input.VITE_PUBLIC_CONTACT_URL)
		expect(result.config.privacyContactURL).toBeUndefined()
		expect(result.issues).toContainEqual({
			field: 'VITE_PUBLIC_PRIVACY_CONTACT_URL',
			code: 'missing',
		})
	})
	it.each([
		'javascript:alert(1)',
		'data:text/html,<script>synthetic</script>',
		'http://contact.example.invalid',
		'//contact.example.invalid',
		credentialTestURL('contact.example.invalid', false),
		'https://contact.example.invalid:8443',
		'https://contact.example.invalid?prefill=synthetic',
		'https://contact.example.invalid?',
		'https://contact.example.invalid/#synthetic',
		'https://contact.example.invalid/encoded%0a',
		'https://contact.example.invalid/../private',
		'https://127.0.0.1',
		'https://localhost',
		'https://contact.example.invalid/\nunsafe',
		'mailto:a@example.invalid?subject=synthetic',
		'mailto:a@example.invalid,b@example.invalid',
		'mailto:a@example.invalid#synthetic',
		'mailto:a%0d%0a@example.invalid',
		'mailto:a..b@example.invalid',
	])('rejects unsafe contact destination %s without echoing it', (url) => {
		const result = validatePublicPolicyConfig({
			...input,
			VITE_PUBLIC_CONTACT_URL: url,
			VITE_PUBLIC_PRIVACY_CONTACT_URL: url,
		})
		expect(result.config.contactURL).toBeUndefined()
		expect(result.config.privacyContactURL).toBeUndefined()
		expect(result.issues).toEqual([
			{ field: 'VITE_PUBLIC_CONTACT_URL', code: 'invalid' },
			{ field: 'VITE_PUBLIC_PRIVACY_CONTACT_URL', code: 'invalid' },
		])
	})
	it.each([
		'<script>synthetic</script>',
		'Synthetic\nName',
		'\u202eSynthetic',
		'A'.repeat(161),
		'[要確定]',
	])('keeps unsafe public text unset', (value) => {
		const result = validatePublicPolicyConfig({
			...input,
			VITE_PUBLIC_OPERATOR_NAME: value,
			VITE_PUBLIC_JURISDICTION: value,
		})
		expect(result.config.operatorName).toBeUndefined()
		expect(result.config.jurisdiction).toBeUndefined()
	})
	it.each([
		'2026-02-29',
		'1900-02-29',
		'2026-04-31',
		'2026-13-01',
		'2026-01-00',
		'0000-01-01',
		'26-01-01',
		'2026-1-1',
		'2026-10-07T00:00:00Z',
	])('rejects impossible or ambiguous calendar date %s', (date) => {
		const result = validatePublicPolicyConfig({
			...input,
			VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: date,
		})
		expect(result.config.privacyEffectiveDate).toBeUndefined()
		expect(result.config.termsEffectiveDate).toBe('2026-10-08')
		expect(result.issues).toEqual([
			{ field: 'VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE', code: 'invalid' },
		])
	})
	it.each([
		'2000-02-29',
		'2024-02-29',
		'0001-01-01',
		'0099-12-31',
	])('accepts real calendar date %s independently of JS year coercion', (date) => {
		expect(
			validatePublicPolicyConfig({
				...input,
				VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: date,
			}).config.privacyEffectiveDate,
		).toBe(date)
	})
	it.each([
		'VITE_OAUTH_CLIENT_SECRET',
		'channelID',
		'supportChallenge',
		'approved',
	])('rejects unexpected secret, user, proof or approval fields %s', (field) => {
		const result = validatePublicPolicyConfig({
			...input,
			[field]: 'synthetic-private-value',
		})
		expect(result).toEqual({
			config: {},
			issues: [{ field: 'input', code: 'unknown-field' }],
		})
		expect(JSON.stringify(result)).not.toContain('synthetic-private-value')
	})
	it.each([
		null,
		[],
		'synthetic',
		1,
	])('rejects malformed input envelopes', (value) => {
		expect(validatePublicPolicyConfig(value)).toEqual({
			config: {},
			issues: [{ field: 'input', code: 'invalid' }],
		})
	})
	it('reads only the six explicitly public build fields', () => {
		for (const [key, value] of Object.entries(input)) vi.stubEnv(key, value)
		vi.stubEnv('VITE_OAUTH_CLIENT_SECRET', 'synthetic-private-value')
		vi.stubEnv('VITE_SUPPORT_CHALLENGE', 'synthetic-proof-value')
		expect(readPublicPolicyConfig()).toEqual(validatePublicPolicyConfig(input))
	})
})
