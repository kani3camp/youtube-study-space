import {
	createMemoryHistory,
	createRootRoute,
	createRouter,
	RouterContextProvider,
} from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ContactPage } from './contact'
import { PUBLIC_POLICY_FIELDS } from './public-policy-config'

beforeEach(() => {
	for (const field of PUBLIC_POLICY_FIELDS) vi.stubEnv(field, undefined)
})
afterEach(() => vi.unstubAllEnvs())

function markup(props: Parameters<typeof ContactPage>[0] = {}) {
	const router = createRouter({
		routeTree: createRootRoute(),
		history: createMemoryHistory(),
	})
	return renderToStaticMarkup(
		<RouterContextProvider router={router}>
			<ContactPage {...props} />
		</RouterContextProvider>,
	)
}

describe('contact destinations and request guidance', () => {
	it('keeps both unconfigured destinations pending without inventing an intake form', () => {
		const html = markup()
		expect(html.match(/窓口を準備中です/g)).toHaveLength(2)
		expect(html).not.toContain('class="button primary"')
		expect(html).not.toContain('mailto:')
		expect(html).not.toContain('<form')
		expect(html).not.toContain('<input')
		expect(html).not.toContain('依頼の本人確認が完了しました')
		expect(html).toContain('href="/"')
		expect(html).toContain('href="/privacy"')
		expect(html).not.toContain('href="/login')
	})
	it('does not reuse a general destination for private data requests', () => {
		vi.stubEnv(
			'VITE_PUBLIC_CONTACT_URL',
			'https://contact.example.invalid/general',
		)
		const html = markup()
		expect(html).toContain('一般の窓口へ連絡する')
		expect(html).not.toContain('プライバシーの窓口へ連絡する')
		expect(
			html.match(/href="https:\/\/contact.example.invalid\/general"/g),
		).toHaveLength(1)
		expect(html.match(/窓口を準備中です/g)).toHaveLength(1)
	})
	it('makes an independently configured privacy mailbox usable without a general destination', () => {
		vi.stubEnv(
			'VITE_PUBLIC_PRIVACY_CONTACT_URL',
			'mailto:privacy@example.invalid',
		)
		const html = markup()
		expect(html).toContain('href="mailto:privacy@example.invalid"')
		expect(html).toContain(
			'class="contact-address">privacy@example.invalid</span>',
		)
		expect(html).toContain('メールアプリが開かない場合は')
		expect(html).not.toContain('一般の窓口へ連絡する')
		expect(html.match(/窓口を準備中です/g)).toHaveLength(1)
	})
	it('shows both configured destinations as separate actions with no user data added', () => {
		vi.stubEnv('VITE_PUBLIC_CONTACT_URL', 'mailto:general@example.invalid')
		vi.stubEnv(
			'VITE_PUBLIC_PRIVACY_CONTACT_URL',
			'https://contact.example.invalid/privacy',
		)
		const html = markup({ proofRef: 'synthetic-private-proof' })
		expect(html.match(/class="button primary"/g)).toHaveLength(2)
		expect(html).toContain('href="mailto:general@example.invalid"')
		expect(html).toContain('href="https://contact.example.invalid/privacy"')
		expect(html).not.toContain('窓口を準備中です')
		expect(html).toContain('target="_blank" rel="noopener noreferrer"')
		for (const href of html.matchAll(/href="([^"]+)"/g)) {
			expect(href[1]).not.toContain('synthetic-private-proof')
			expect(href[1]).not.toContain('?')
		}
	})
	it('leaves unsafe configured destinations pending', () => {
		vi.stubEnv('VITE_PUBLIC_CONTACT_URL', 'javascript:synthetic')
		vi.stubEnv(
			'VITE_PUBLIC_PRIVACY_CONTACT_URL',
			'https://contact.example.invalid/?supportChallenge=synthetic',
		)
		const html = markup()
		expect(html.match(/窓口を準備中です/g)).toHaveLength(2)
		expect(html).not.toContain('synthetic')
		expect(html).not.toContain('supportChallenge')
	})
	it('explains human follow-up and preserves proof success independently of intake availability', () => {
		const html = markup({ proofRef: 'synthetic<&proof' })
		expect(html).toContain('依頼の本人確認が完了しました')
		expect(html).toContain('synthetic&lt;&amp;proof')
		expect(html).toContain(
			'この番号は画面を離れたり、再読み込みすると表示されなくなります。',
		)
		expect(html).toContain(
			'削除やログインの解除は、この画面では実行していません。',
		)
		expect(html).toContain('受付時の連絡先へお知らせします。')
		expect(html).toContain('回答を受け取れる連絡先')
		expect(html).toContain('ログインや本人確認を利用できない場合も')
		expect(html).toContain('ログインを、ご依頼の条件にはしません。')
	})
})
