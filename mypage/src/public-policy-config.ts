// D03の公開値だけを使う。公開ビルド入力であり、
// 認証情報・申請の証拠・ユーザーデータ・文面承認の証拠ではない。
export const PUBLIC_POLICY_FIELDS = [
	'VITE_PUBLIC_OPERATOR_NAME',
	'VITE_PUBLIC_CONTACT_URL',
	'VITE_PUBLIC_PRIVACY_CONTACT_URL',
	'VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE',
	'VITE_PUBLIC_TERMS_EFFECTIVE_DATE',
	'VITE_PUBLIC_JURISDICTION',
] as const

export type PublicPolicyField = (typeof PUBLIC_POLICY_FIELDS)[number]
export type PublicPolicyConfig = {
	// 任意の公開運営者名・サービス名であり、非公開の本名を求めるものではない。
	operatorName?: string
	contactURL?: string
	privacyContactURL?: string
	privacyEffectiveDate?: string
	termsEffectiveDate?: string
	jurisdiction?: string
}
type PublicPolicyResult = {
	config: PublicPolicyConfig
	issues: {
		field: PublicPolicyField | 'input'
		code: 'missing' | 'invalid' | 'unknown-field'
	}[]
}

function publicText(value: string) {
	return (
		value.length <= 160 &&
		!/[\p{Cc}\p{Cf}<>]/u.test(value) &&
		!value.includes('要確定')
	)
}

function calendarDate(value: string) {
	if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
	const [year, month, day] = value.split('-').map(Number)
	const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)
	const days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
	return (
		year > 0 && month >= 1 && month <= 12 && day >= 1 && day <= days[month - 1]
	)
}

function publicContact(value: string) {
	if (value.length > 2048 || /[\s\p{Cc}\p{Cf}<>\\%?#]/u.test(value))
		return false
	const domain = '[a-z0-9](?:[a-z0-9-]*[a-z0-9])?'
	const hostname = new RegExp(`^(?:${domain}\\.)+[a-z]{2,}$`, 'i')
	if (value.startsWith('mailto:')) {
		const recipient = value.slice('mailto:'.length)
		const parts = recipient.split('@')
		return (
			parts.length === 2 &&
			parts[0].length <= 64 &&
			/^[a-z0-9](?:[a-z0-9._+-]*[a-z0-9])?$/i.test(parts[0]) &&
			!parts[0].includes('..') &&
			hostname.test(parts[1]) &&
			!/[?#]/.test(recipient)
		)
	}
	try {
		const url = new URL(value)
		return (
			url.protocol === 'https:' &&
			!url.username &&
			!url.password &&
			!url.port &&
			!url.search &&
			!url.hash &&
			hostname.test(url.hostname) &&
			(value === url.origin || value === url.href)
		)
	} catch {
		return false
	}
}

// 公開専用の厳格な形式で、設定の展開によるサーバーシークレットや利用者・申請項目の漏えいを防ぐ。
// エラーにはフィールド名だけを含め、値は含めない。
// 有効な値が一部だけでもプレビューでき、欠落値・不正値は未設定のままにする。
export function validatePublicPolicyConfig(input: unknown): PublicPolicyResult {
	if (typeof input !== 'object' || input === null || Array.isArray(input))
		return { config: {}, issues: [{ field: 'input', code: 'invalid' }] }
	if (
		Object.keys(input).some(
			(key) => !PUBLIC_POLICY_FIELDS.some((x) => x === key),
		)
	)
		return { config: {}, issues: [{ field: 'input', code: 'unknown-field' }] }
	const values = input as Record<string, unknown>
	const result: PublicPolicyResult = { config: {}, issues: [] }
	const fields = [
		['VITE_PUBLIC_OPERATOR_NAME', 'operatorName', publicText],
		['VITE_PUBLIC_CONTACT_URL', 'contactURL', publicContact],
		['VITE_PUBLIC_PRIVACY_CONTACT_URL', 'privacyContactURL', publicContact],
		[
			'VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE',
			'privacyEffectiveDate',
			calendarDate,
		],
		['VITE_PUBLIC_TERMS_EFFECTIVE_DATE', 'termsEffectiveDate', calendarDate],
		['VITE_PUBLIC_JURISDICTION', 'jurisdiction', publicText],
	] as const
	for (const [field, target, valid] of fields) {
		const raw = Object.hasOwn(values, field) ? values[field] : undefined
		if (raw === undefined || (typeof raw === 'string' && raw.trim() === '')) {
			if (field !== 'VITE_PUBLIC_OPERATOR_NAME')
				result.issues.push({ field, code: 'missing' })
		} else if (
			typeof raw !== 'string' ||
			/[\p{Cc}\p{Cf}]/u.test(raw) ||
			!valid(raw.trim())
		) {
			result.issues.push({ field, code: 'invalid' })
		} else {
			result.config[target] = raw.trim()
		}
	}
	return result
}

// 明示した項目だけを読み、Viteの全環境変数をページへコピーしない。
export function readPublicPolicyConfig(): PublicPolicyResult {
	const env = import.meta.env
	return validatePublicPolicyConfig({
		VITE_PUBLIC_OPERATOR_NAME: env.VITE_PUBLIC_OPERATOR_NAME,
		VITE_PUBLIC_CONTACT_URL: env.VITE_PUBLIC_CONTACT_URL,
		VITE_PUBLIC_PRIVACY_CONTACT_URL: env.VITE_PUBLIC_PRIVACY_CONTACT_URL,
		VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: env.VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE,
		VITE_PUBLIC_TERMS_EFFECTIVE_DATE: env.VITE_PUBLIC_TERMS_EFFECTIVE_DATE,
		VITE_PUBLIC_JURISDICTION: env.VITE_PUBLIC_JURISDICTION,
	})
}
