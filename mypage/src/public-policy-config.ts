// プライバシーポリシー・利用規約の公開値。本文と同じくソースで管理し、
// 変更はソースレビューと再ビルドで反映する（環境変数や公開マニフェストでは上書きしない）。

// 利用者が同意する本文の版。サーバーは同意時の版を記録し、現在の版と異なる場合は再同意を求める。
// 本文を改訂して再同意を求めるときに上げ、system/core/mypage/policy.go の定数も同じ値にする
// （public-policy-config.test.ts で照合する）。
export const POLICY_VERSIONS = { privacy: 'v1', terms: 'v1' } as const

// 両文書の施行日（YYYY-MM-DD）。初回本番公開日とする。
// 未設定のときは施行日を表示しない。本番向け設定の事前検査では有効な日付の設定を必須とする。
export const POLICY_EFFECTIVE_DATE: string | undefined = '2026-11-21'

export function isValidCalendarDate(value: unknown): value is string {
	if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value))
		return false
	const [year, month, day] = value.split('-').map(Number)
	const date = new Date(Date.UTC(year, month - 1, day))
	return (
		date.getUTCFullYear() === year &&
		date.getUTCMonth() === month - 1 &&
		date.getUTCDate() === day
	)
}

// 運営者の公開表記。
export const OPERATOR_NAME = 'らいど開発'

export type PublicPolicyConfig = {
	operatorName: string
	// 一般・プライバシーの両方で使う共通のお問い合わせ先。
	contactURL?: string
	effectiveDate?: string
}

// 共通のお問い合わせフォームの公開回答用URL。Firebaseの接続先ごとに固定する。
const contactForms = {
	'test-youtube-study-space':
		'https://docs.google.com/forms/d/e/1FAIpQLScKyaOQSxKIxs1znN72MlrDwHmJoQ94MnEuUK9UKD_fgqHEzg/viewform',
	'youtube-study-space':
		'https://docs.google.com/forms/d/e/1FAIpQLSfTvdRD8r3QPOBMNjNwixzaITe5ca-BPst--ZOoK1aFZXAy2Q/viewform',
} as const

// 未知・未設定のプロジェクトでは窓口を推測せず、お問い合わせボタンを表示しない。
export function contactFormForProject(project: unknown): string | undefined {
	return project === 'test-youtube-study-space' ||
		project === 'youtube-study-space'
		? contactForms[project]
		: undefined
}

export function publicPolicyConfig(
	project: unknown = import.meta.env.VITE_FIREBASE_PROJECT_ID,
): PublicPolicyConfig {
	return {
		operatorName: OPERATOR_NAME,
		contactURL: contactFormForProject(project),
		effectiveDate: POLICY_EFFECTIVE_DATE,
	}
}
