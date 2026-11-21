import {
	createMemoryHistory,
	createRootRoute,
	createRouter,
	RouterContextProvider,
} from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ContactPage } from './contact'

beforeEach(() => {
	vi.stubEnv('VITE_FIREBASE_PROJECT_ID', 'synthetic-unknown-project')
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
	it('shows no destination actions when unconfigured without inventing an intake form', () => {
		const html = markup()
		expect(html).not.toContain('窓口を準備中です')
		expect(html).not.toContain('class="button primary"')
		expect(html).not.toContain('mailto:')
		expect(html).not.toContain('<form')
		expect(html).not.toContain('<input')
		expect(html).not.toContain('依頼の本人確認が完了しました')
		expect(html).toContain('href="/"')
		expect(html).toContain('href="/privacy"')
		expect(html).not.toContain('href="/login')
	})
	it('links both cards to the fixed shared form for known projects and keeps proof out of the form URL', () => {
		vi.stubEnv('VITE_FIREBASE_PROJECT_ID', 'test-youtube-study-space')
		const html = markup({ proofRef: 'synthetic-private-proof' })
		expect(html).toContain('synthetic-private-proof')
		expect(html.match(/class="button primary"/g)).toHaveLength(2)
		const hrefs = [...html.matchAll(/href="([^"]+)"/g)].map((match) => match[1])
		expect(
			hrefs.filter((href) => href.startsWith('https://docs.google.com/')),
		).toEqual([
			'https://docs.google.com/forms/d/e/1FAIpQLScKyaOQSxKIxs1znN72MlrDwHmJoQ94MnEuUK9UKD_fgqHEzg/viewform',
			'https://docs.google.com/forms/d/e/1FAIpQLScKyaOQSxKIxs1znN72MlrDwHmJoQ94MnEuUK9UKD_fgqHEzg/viewform',
		])
		expect(html.match(/お問い合わせフォームを開く/g)).toHaveLength(2)
		expect(html).toContain('href="/privacy">プライバシーポリシー</a>')
		expect(html).toContain('運営者・サービスの公開表記：らいど開発')
		for (const href of hrefs) {
			expect(href).not.toContain('synthetic-private-proof')
			expect(href).not.toContain('?')
		}
	})
	it('ignores the retired contact URL build setting for unknown projects', () => {
		vi.stubEnv('VITE_PUBLIC_CONTACT_URL', 'mailto:retired@example.invalid')
		const html = markup()
		expect(html).not.toContain('class="button primary"')
		expect(html).not.toContain('retired@example.invalid')
	})
	it('explains human follow-up and preserves proof success independently of intake availability', () => {
		const html = markup({ proofRef: 'synthetic<&proof' })
		expect(html).toContain('依頼の本人確認が完了しました')
		expect(html).toContain('synthetic&lt;&amp;proof')
		expect(html).toContain(
			'この番号は、画面を離れるか再読み込みすると表示されなくなります。',
		)
		expect(html).toContain(
			'削除やログインの解除は、この画面では実行していません。',
		)
		expect(html).toContain('受付時の連絡先へお知らせします。')
		expect(html).toContain('回答を受け取れる連絡先')
		expect(html).toContain('認証手続きを利用できない場合も')
		expect(html).toContain('ログインを、ご依頼の条件にはしません。')
	})
})
