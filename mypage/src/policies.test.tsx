import {
	createMemoryHistory,
	createRootRoute,
	createRouter,
	RouterContextProvider,
} from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp } from './app'
import { PrivacyPolicy, PublicPolicyContact, TermsOfUse } from './policies'
import { PUBLIC_POLICY_FIELDS } from './public-policy-config'
import { BrowserRuntime } from './runtime'
import { renderStaticPolicies } from './static-policies'

beforeEach(() => vi.stubEnv('VITE_FIREBASE_PROJECT_ID', undefined))
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

describe('public policy rendering', () => {
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
	it('renders the complete policy documents and their links in static HTML', () => {
		for (const [field, value] of Object.entries(publicValues))
			vi.stubEnv(field, value)
		const [privacy, terms] = renderStaticPolicies()
		expect(privacy).toMatchObject({
			filename: 'privacy.html',
			title: 'プライバシーポリシー',
		})
		expect(terms).toMatchObject({ filename: 'terms.html', title: '利用規約' })
		expect(privacy.body).toContain(markup(<PrivacyPolicy />))
		expect(privacy.body).toContain('オンライン作業部屋（Online Study Space')
		const deletion = privacy.body
			.split('<h2 id="data-deletion">')[1]
			?.split('<h2')[0]
		expect(deletion).toContain('href="#contact">第16節の専用窓口</a>')
		expect(deletion).not.toContain('href="/contact"')
		expect(privacy.body).toContain(
			'<h2 id="contact">第16節 運営者・お問い合わせ</h2>',
		)
		expect(terms.body).toContain(markup(<TermsOfUse />))
		for (const { body } of [privacy, terms]) {
			expect(body).toContain('href="/contact"')
			expect(body).not.toContain('<script')
		}
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
	it('shows public policy text without release notes or unconfigured values', () => {
		const privacy = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		const terms = markup(<TermsOfUse publicPolicyInputs={{}} />)
		for (const html of [privacy, terms]) {
			expect(html).not.toMatch(/公開前|公開承認済み|要確定|遅滞なく|非公開検証/)
			expect(html).not.toContain('施行日')
			expect(html).toContain('href="/contact">一般のお問い合わせ</a>')
			expect(html).not.toContain('<time')
		}
		expect(privacy.match(/<h2(?: id="[^"]+")?>第[0-9]+節/g)).toHaveLength(17)
		expect(terms.match(/<h2>/g)).toHaveLength(18)
		expect(terms).toContain(
			'本規約は日本法に準拠し、日本法に従って解釈されます。',
		)
		expect(terms).not.toContain('専属的合意管轄裁判所')
		expect(terms).toContain('故意または重過失')
		expect(terms).toContain('当事者間の合意に優先して適用される法令')
		expect(terms).toContain('その規定が本規約に優先します。')
		expect(terms).toContain('href="/privacy"')
		expect(terms).not.toContain('app.notion.com')
	})
	it('preserves receipt-based deletion, purpose-bound proof and metadata retention distinctions', () => {
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain('遅くとも受付から7日以内に削除します。')
		expect(html).toContain('この期限は依頼の受付時点から数えます。')
		expect(html).toContain(
			'通常のログインとは別にGoogle（YouTube）の認証手続きを行っていただきます。',
		)
		expect(html).toContain(
			'認証操作だけでデータの削除やすべてのログイン状態の解除が実行されることはありません。',
		)
		expect(html).toContain('取得から30日以内に更新または削除します。')
		expect(html).toContain(
			'利用のないチャンネルについても、期限内の削除の対象とします。',
		)
		expect(html).toContain('マイページでのログイン状態がすぐには終了しない場合')
		expect(html).toContain(
			'保存データ全体の削除をご希望の場合は、<a href="#data-deletion">第12節</a>の方法でご依頼ください。',
		)
	})
	it('explains how to request action after a Google disconnect', () => {
		const html = markup(<PrivacyPolicy publicPolicyInputs={{}} />)
		expect(html).toContain(
			'連携解除後のログイン状態の解除や保存データの削除をご希望の場合は、<a href="#contact">第16節の専用窓口</a>へご連絡ください。',
		)
		expect(html).toContain(
			'チャンネルIDと作業履歴は、Google連携の解除だけでは削除されません。',
		)
		expect(html).toContain(
			'マイページのアカウント情報とチャンネルとの関連付けを削除',
		)
	})
	it('explains fresh YouTube authorization and individual guidance in the privacy policy', () => {
		const Policy = PrivacyPolicy
		const html = markup(<Policy publicPolicyInputs={{}} />)
		expect(html).toContain(
			'通常のログインとは別にGoogle（YouTube）の認証手続きを行っていただきます。',
		)
		expect(html).toContain(
			'認証操作だけでデータの削除やすべてのログイン状態の解除が実行されることはありません。',
		)
		expect(html).toContain(
			'Google連携を解除済みなどでログインや本人確認を利用できない場合も',
		)
		expect(html).toContain('個別の確認方法をご案内します。')
		expect(html).toMatch(
			/マイページへのログイン[もは][^。]*条件ではありません。/,
		)
	})
	it('links the terms to request guidance and explains public chat', () => {
		const html = markup(<TermsOfUse publicPolicyInputs={{}} />)
		expect(html).toContain(
			'YouTubeのライブチャットに入力した内容は他の視聴者にも公開されます。',
		)
		expect(html).toContain('href="/contact">一般のお問い合わせ</a>')
		expect(html).toContain('運営者の軽過失により損害賠償責任を負う場合')
		expect(html).toContain('親権者等の法定代理人の同意')
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
		const html = markup(
			<>
				<PrivacyPolicy />
				<TermsOfUse />
			</>,
		)
		expect(
			html.split(`href="https://docs.google.com/forms/d/e/${form}/viewform"`),
		).toHaveLength(3)
		expect(html).toContain('個人情報の開示・削除などの専用フォーム')
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
	it('shows configured destinations, effective dates and jurisdiction', () => {
		const privacy = markup(<PrivacyPolicy publicPolicyInputs={publicValues} />)
		const terms = markup(<TermsOfUse publicPolicyInputs={publicValues} />)
		for (const html of [privacy, terms]) {
			expect(html).toContain('Synthetic Service &amp; &quot;Friends&quot;')
			expect(html).toContain('href="/contact">一般のお問い合わせ</a>')
			expect(html).toContain('href="mailto:privacy@example.invalid"')
			expect(html).not.toMatch(/公開前|公開承認済み|要確定|遅滞なく|非公開検証/)
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
		expect(html).toContain('個人情報・プライバシーの専用窓口は準備中です。')
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
		expect(html.match(/<time/g)).toHaveLength(1)
		expect(html).not.toContain('専属的合意管轄裁判所')
	})
	it('renders only explicitly configured public Vite values', () => {
		for (const field of PUBLIC_POLICY_FIELDS) vi.stubEnv(field, undefined)
		const defaultHTML = markup(<PrivacyPolicy />)
		expect(defaultHTML).not.toContain('<time')
		expect(defaultHTML).not.toContain('mailto:')
		for (const [field, value] of Object.entries(publicValues))
			vi.stubEnv(field, value)
		vi.stubEnv('VITE_PRIVATE_OPERATOR_NAME', 'synthetic-private-name')
		const configuredHTML = markup(<PrivacyPolicy />)
		expect(configuredHTML).toContain('href="mailto:privacy@example.invalid"')
		expect(configuredHTML).toContain(
			'<time dateTime="2026-10-07">2026-10-07</time>',
		)
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
			'通常のログインとは別にGoogleの認証手続きで対象のYouTubeチャンネルを確認します。',
		)
		expect(html).toContain(
			'Google連携を解除済みなどで認証手続きを利用できない場合も',
		)
		expect(html).toContain('個別の確認方法をご案内します。')
		expect(html).not.toContain('supportChallenge')
		expect(request).not.toHaveBeenCalled()
	})
	it('preserves a support failure when the client router validates normalized search again', () => {
		const runtime = new BrowserRuntime(
			null,
			{ privacy: 'synthetic-v1', terms: 'synthetic-v1' },
			vi.fn<typeof fetch>(),
			'LOGIN_UNAVAILABLE',
		)
		const router = createApp(runtime, createMemoryHistory())
		const validate = router.routesById['/contact'].options.validateSearch
		if (typeof validate !== 'function')
			throw new Error('Contact search missing')
		const normalized = validate({ supportError: 'oauth_denied' })
		expect(normalized).toEqual({ supportError: true })
		expect(validate(normalized)).toEqual(normalized)
		expect(validate({ supportError: 'private-provider-detail' })).toEqual({
			supportError: undefined,
		})
	})
	it('shows only allowlisted support OAuth failures after contact route search normalization', async () => {
		const renderContact = async (entry: string) => {
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
			await router.load()
			return renderToStaticMarkup(
				<RouterContextProvider router={router}>
					<Contact />
				</RouterContextProvider>,
			)
		}
		const failure = await renderContact('/contact?supportError=oauth_denied')
		expect(failure).toContain('YouTubeでの本人確認を完了できませんでした。')
		expect(failure).toContain('受付窓口からもう一度お手続きください。')
		const untrusted = await renderContact(
			'/contact?supportError=private-provider-detail',
		)
		expect(untrusted).not.toContain('本人確認を完了できませんでした。')
		expect(untrusted).not.toContain('private-provider-detail')
	})
})
