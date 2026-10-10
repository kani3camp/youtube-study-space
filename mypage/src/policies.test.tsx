import {
	createMemoryHistory,
	createRootRoute,
	createRouter,
	RouterContextProvider,
} from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp } from './app'
import { PrivacyPolicy, PublicPolicyContact, TermsOfUse } from './policies'
import { PUBLIC_POLICY_FIELDS } from './public-policy-config'
import { BrowserRuntime } from './runtime'

afterEach(() => vi.unstubAllEnvs())

function markup(children: ReactNode) {
	const router = createRouter({
		routeTree: createRootRoute(),
		history: createMemoryHistory(),
	})
	return renderToStaticMarkup(
		<RouterContextProvider router={router}>{children}</RouterContextProvider>,
	)
}
const publicValues = {
	VITE_PUBLIC_OPERATOR_NAME: 'Synthetic Service & "Friends"',
	VITE_PUBLIC_CONTACT_URL: 'https://contact.example.invalid/general',
	VITE_PUBLIC_PRIVACY_CONTACT_URL: 'mailto:privacy@example.invalid',
	VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: '2026-10-07',
	VITE_PUBLIC_TERMS_EFFECTIVE_DATE: '2026-10-08',
	VITE_PUBLIC_JURISDICTION: '合成テスト裁判所',
}

describe('public policy draft rendering', () => {
	it('keeps both full drafts and pending values available without a login', () => {
		const privacy = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		const terms = markup(<TermsOfUse publicPolicyInputs={{}} />)
		for (const html of [privacy, terms]) {
			expect(html).toContain('class="notice" role="status"')
			expect(html).toContain('公開前の確認用文面です。')
			expect(html).toContain('公開承認済みの文面ではありません。')
			expect(html).toContain('[要確定：施行日]')
			expect(html).toContain(
				'窓口への請求に応じ、法令に従い遅滞なく回答する方針です。',
			)
			expect(html).not.toContain('<time')
		}
		expect(privacy.match(/<h2>/g)).toHaveLength(17)
		expect(terms.match(/<h2>/g)).toHaveLength(18)
		expect(terms).toContain(
			'本規約は日本法に準拠し、日本法に従って解釈されます。',
		)
		expect(terms).toContain('[要確定：第一審の専属的合意管轄裁判所]')
		expect(terms).toContain('故意または重過失')
		expect(terms).toContain('消費者契約法その他の強行法規')
		expect(terms).toContain('href="/privacy"')
		expect(terms).not.toContain('app.notion.com')
	})
	it('preserves receipt-based deletion, purpose-bound proof and metadata retention distinctions', () => {
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain(
			'削除意思を受け付けた時点から可能な限り速やかに、遅くとも7暦日以内',
		)
		expect(html).toContain(
			'本人確認の完了時点へ期限の起算を無条件に繰り下げません。',
		)
		expect(html).toContain('通常のログイン成功を別の依頼へ流用しません。')
		expect(html).toContain(
			'本人確認の完了だけで削除や全ログインの解除を実行するものではありません。',
		)
		expect(html).toContain('30暦日以内に更新または削除する方針')
		expect(html).toContain('未利用のチャンネルも期限内の削除対象')
		expect(html).toContain('既存のFirebase sessionが直ちに終了しない場合')
		expect(html).toContain('本サービスに保存された作業履歴等の削除とは別の操作')
	})
	it('keeps explicit Analytics opt-in separate from necessary Firebase communication', () => {
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain('本サービスのAnalyticsはdefaultでOFF')
		expect(html).toContain('利用者が明示的に「Analyticsを許可」した場合に限り')
		expect(html).toContain(
			'許可前に発生したイベントを、後から遡ってGA4へ送信しません。',
		)
		expect(html).toContain(
			'AnalyticsをOFFにしてもサービス利用に必要な通信は行われます。',
		)
		expect(html).toContain('Google Signals、User-IDは使いません。')
	})
	it('shows separately supplied destinations and effective dates while remaining draft', () => {
		const privacy = markup(<PrivacyPolicy publicPolicyInputs={publicValues} />)
		const terms = markup(<TermsOfUse publicPolicyInputs={publicValues} />)
		for (const html of [privacy, terms]) {
			expect(html).toContain('Synthetic Service &amp; &quot;Friends&quot;')
			expect(html).toContain('href="https://contact.example.invalid/general"')
			expect(html).toContain('href="mailto:privacy@example.invalid"')
			expect(html).toContain('公開承認済みの文面ではありません。')
		}
		expect(privacy).toContain('<time dateTime="2026-10-07">2026-10-07</time>')
		expect(terms).toContain('<time dateTime="2026-10-08">2026-10-08</time>')
		expect(terms).toContain('合成テスト裁判所')
	})
	it('leaves privacy request reception pending with only a general contact link', () => {
		const html = markup(
			<PublicPolicyContact
				publicPolicyInputs={{
					VITE_PUBLIC_CONTACT_URL: publicValues.VITE_PUBLIC_CONTACT_URL,
				}}
			/>,
		)
		expect(html).toContain('一般のお問い合わせ')
		expect(html).toContain(
			'個人情報・プライバシーに関する返信可能な専用窓口は、公開前に確定して掲載します。',
		)
		expect(html.match(/href=/g)).toHaveLength(1)
		expect(html).not.toContain('本人確認が完了')
	})
	it('never renders unsafe supplied values or falls back to invented contact and legal values', () => {
		const values = {
			...publicValues,
			VITE_PUBLIC_OPERATOR_NAME: '<img src=x onerror=synthetic>',
			VITE_PUBLIC_CONTACT_URL: 'javascript:synthetic',
			VITE_PUBLIC_PRIVACY_CONTACT_URL:
				'https://contact.example.invalid/?supportChallenge=synthetic-proof',
			VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE: '2026-02-29',
			VITE_PUBLIC_JURISDICTION: 'synthetic\u202e',
		}
		const html = markup(
			<>
				<PrivacyPolicy publicPolicyInputs={values} />
				<TermsOfUse publicPolicyInputs={values} />
			</>,
		)
		expect(html).not.toContain('synthetic')
		expect(html).not.toContain('<img')
		expect(html).not.toContain('javascript:')
		expect(html).toContain('[要確定：施行日]')
		expect(html).toContain('[要確定：第一審の専属的合意管轄裁判所]')
	})
	it('connects only explicit public Vite values and keeps a no-config draft default', () => {
		for (const field of PUBLIC_POLICY_FIELDS) vi.stubEnv(field, undefined)
		const defaultHTML = markup(<PrivacyPolicy />)
		expect(defaultHTML).toContain('[要確定：施行日]')
		expect(defaultHTML).not.toContain('mailto:')
		for (const [field, value] of Object.entries(publicValues))
			vi.stubEnv(field, value)
		vi.stubEnv('VITE_PRIVATE_OPERATOR_NAME', 'synthetic-private-name')
		const configuredHTML = markup(<PrivacyPolicy />)
		expect(configuredHTML).toContain('href="mailto:privacy@example.invalid"')
		expect(configuredHTML).toContain('公開承認済みの文面ではありません。')
		expect(configuredHTML).not.toContain('synthetic-private-name')
	})
	it('shows configured contact destinations without opening a login or executing a request', () => {
		for (const [field, value] of Object.entries(publicValues))
			vi.stubEnv(field, value)
		const request = vi.fn<typeof fetch>()
		const runtime = new BrowserRuntime(
			null,
			{ privacy: 'synthetic-v1', terms: 'synthetic-v1' },
			request,
			'LOGIN_UNAVAILABLE',
		)
		const router = createApp(runtime, createMemoryHistory())
		const Contact = router.routesById['/contact'].options.component
		if (!Contact) throw new Error('Contact route missing')
		const html = renderToStaticMarkup(
			<RouterContextProvider router={router}>
				<Contact />
			</RouterContextProvider>,
		)
		expect(html).toContain('href="https://contact.example.invalid/general"')
		expect(html).toContain('href="mailto:privacy@example.invalid"')
		expect(html).toContain('本人確認付きの請求窓口は準備中です。')
		expect(html).toContain('通常のログインだけで依頼を実行しません。')
		expect(html).toContain('ログインや本人確認を利用できない場合も')
		expect(html).not.toContain('supportChallenge')
		expect(request).not.toHaveBeenCalled()
	})
})
