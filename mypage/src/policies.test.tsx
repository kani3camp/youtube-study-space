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
	it.each([
		['/privacy_policy', false],
		['/privacy_policy', true],
		[
			'/privacy_policy?next=https://example.invalid&code=synthetic#synthetic',
			false,
		],
		[
			'/privacy_policy?next=https://example.invalid&code=synthetic#synthetic',
			true,
		],
	] as const)('replaces the published OAuth policy URL %s with the public privacy route (maintenance=%s)', async (entry, maintenance) => {
		const request = vi.fn<typeof fetch>()
		const runtime = new BrowserRuntime(
			null,
			{ privacy: 'synthetic-v1', terms: 'synthetic-v1' },
			request,
			'LOGIN_UNAVAILABLE',
		)
		if (maintenance) runtime.maintenance.markActive()
		const history = createMemoryHistory({ initialEntries: ['/', entry] })
		const router = createApp(runtime, history)
		router.update({ isServer: false, origin: 'https://example.invalid' })
		await router.load()
		expect(router.state.location.href).toBe('/privacy')
		expect(history.length).toBe(2)
		expect(router.state.matches.at(-1)?.routeId).toBe('/privacy')
		const Policy = router.routesById['/privacy'].options.component
		if (!Policy) throw new Error('Privacy route missing')
		expect(markup(<Policy />)).toContain('<h1>プライバシーポリシー</h1>')
		expect(request).not.toHaveBeenCalled()
	})
	it.each([
		'/privacy',
		'/terms',
		'/contact',
	])('keeps the canonical public policy route available without login: %s', async (entry) => {
		const request = vi.fn<typeof fetch>()
		const runtime = new BrowserRuntime(
			null,
			{ privacy: 'synthetic-v1', terms: 'synthetic-v1' },
			request,
			'LOGIN_UNAVAILABLE',
		)
		const router = createApp(
			runtime,
			createMemoryHistory({ initialEntries: [entry] }),
		)
		await router.load()
		expect(router.state.location.href).toBe(entry)
		expect(router.state.matches.at(-1)?.routeId).toBe(entry)
		expect(request).not.toHaveBeenCalled()
	})
	it('keeps unknown public paths as not found', async () => {
		const runtime = new BrowserRuntime(
			null,
			{ privacy: 'synthetic-v1', terms: 'synthetic-v1' },
			vi.fn<typeof fetch>(),
			'LOGIN_UNAVAILABLE',
		)
		const router = createApp(
			runtime,
			createMemoryHistory({ initialEntries: ['/unknown-policy'] }),
		)
		await router.load()
		expect(router.state.location.href).toBe('/unknown-policy')
		expect(router.state.matches.some((match) => match.globalNotFound)).toBe(
			true,
		)
	})
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
		expect(terms).toContain('当事者間の合意に優先して適用される法令')
		expect(terms).toContain('その規定が本規約に優先します。')
		expect(terms).toContain('href="/privacy"')
		expect(terms).not.toContain('app.notion.com')
	})
	it('preserves receipt-based deletion, purpose-bound proof and metadata retention distinctions', () => {
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain('可能な限り速やかに、遅くとも受付から7暦日以内')
		expect(html).toContain(
			'期限は受付時点を基準とし、本人確認が終わるまで一律に延長することはありません。',
		)
		expect(html).toContain(
			'通常のログインを別の依頼の本人確認に流用することはありません。',
		)
		expect(html).toContain(
			'本人確認の画面で削除やすべてのログイン状態の解除が実行されることはありません。',
		)
		expect(html).toContain('30暦日以内に更新または削除する方針')
		expect(html).toContain(
			'利用のないチャンネルについても、期限内の削除の対象とします。',
		)
		expect(html).toContain('マイページでのログイン状態がすぐには終了しない場合')
		expect(html).toContain(
			'保存データ全体の削除をご希望の場合は、第12章の方法でご依頼ください。',
		)
	})
	it('links both policies to public contact after a Google disconnect', () => {
		for (const Policy of [PrivacyPolicy, TermsOfUse]) {
			const html = markup(<Policy publicPolicyInputs={{}} />)
			expect(html).toContain(
				'本サービスとの連携を解除した場合は、速やかに<a href="/contact">お問い合わせ窓口</a>までご連絡ください。',
			)
		}
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain('チャンネルIDと作業履歴は保持します。')
	})
	it.each([
		[
			'test-youtube-study-space',
			'1FAIpQLScKyaOQSxKIxs1znN72MlrDwHmJoQ94MnEuUK9UKD_fgqHEzg',
		],
		[
			'youtube-study-space',
			'1FAIpQLSfTvdRD8r3QPOBMNjNwixzaITe5ca-BPst--ZOoK1aFZXAy2Q',
		],
	])('selects the public request form for %s without login', (project, form) => {
		vi.stubEnv('VITE_FIREBASE_PROJECT_ID', project)
		vi.stubEnv(
			'VITE_PUBLIC_PRIVACY_CONTACT_URL',
			'https://contact.example.invalid/other',
		)
		const html = markup(<PublicPolicyContact />)
		expect(html).toContain(
			`href="https://docs.google.com/forms/d/e/${form}/viewform"`,
		)
		expect(html).not.toContain('contact.example.invalid/other')
	})
	it('keeps explicit Analytics opt-in separate from necessary Firebase communication', () => {
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain('アクセス解析は初期設定で無効です。')
		expect(html).toContain('「アクセス解析を許可」を選んだ場合に限り')
		expect(html).toContain(
			'許可前の閲覧や操作の情報を、許可後にさかのぼって送信することはありません。',
		)
		expect(html).toContain(
			'サービスに必要な通信は、任意のアクセス解析を許可しない場合も行われます。',
		)
		expect(html).toContain(
			'Google シグナル、利用者を識別するGA4のUser-IDは利用しません。',
		)
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
		expect(html).toContain(
			'依頼受付後、本人確認が必要な場合は方法をご案内します。',
		)
		expect(html).toContain('通常のログインだけで依頼を実行しません。')
		expect(html).toContain('Google連携を解除済みでログインできない場合も')
		expect(html).not.toContain('supportChallenge')
		expect(request).not.toHaveBeenCalled()
	})
	it('shows only allowlisted support OAuth failures on the public contact route', () => {
		const renderContact = (entry: string) => {
			const runtime = new BrowserRuntime(
				null,
				{ privacy: 'synthetic-v1', terms: 'synthetic-v1' },
				vi.fn<typeof fetch>(),
				'LOGIN_UNAVAILABLE',
			)
			const router = createApp(
				runtime,
				createMemoryHistory({ initialEntries: [entry] }),
			)
			const Contact = router.routesById['/contact'].options.component
			if (!Contact) throw new Error('Contact route missing')
			return renderToStaticMarkup(
				<RouterContextProvider router={router}>
					<Contact />
				</RouterContextProvider>,
			)
		}
		const failure = renderContact('/contact?supportError=oauth_denied')
		expect(failure).toContain('YouTubeでの本人確認を完了できませんでした。')
		expect(failure).toContain('受付窓口からもう一度お手続きください。')
		const untrusted = renderContact(
			'/contact?supportError=private-provider-detail',
		)
		expect(untrusted).not.toContain('本人確認を完了できませんでした。')
		expect(untrusted).not.toContain('private-provider-detail')
	})
})
