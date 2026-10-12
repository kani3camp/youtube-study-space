import { describe, expect, it, vi } from 'vitest'
import {
	AnalyticsGate,
	ConsentStore,
	consentKey,
	type PreferenceStorage,
	safePage,
} from './privacy'

function storageFixture() {
	const values = new Map<string, string>()
	const storage: PreferenceStorage = {
		getItem: (key) => values.get(key) ?? null,
		setItem: (key, value) => {
			values.set(key, value)
		},
		removeItem: (key) => {
			values.delete(key)
		},
	}
	return { values, storage }
}
describe('privacy consent and analytics boundary', () => {
	it('sends nothing before explicit consent and never replays discarded events', () => {
		const { values, storage } = storageFixture()
		const consent = new ConsentStore(storage)
		const port = { start: vi.fn(), stop: vi.fn(), send: vi.fn() }
		const gate = new AnalyticsGate(consent, port)
		const dispose = gate.mount()
		gate.page('/mypage?uid=synthetic-private#private')
		gate.event('login')
		expect(port.start).not.toHaveBeenCalled()
		expect(port.send).not.toHaveBeenCalled()
		consent.set('granted')
		expect(port.start).toHaveBeenCalledTimes(1)
		expect(port.send).not.toHaveBeenCalled()
		gate.page('/mypage?uid=synthetic-private#private')
		gate.event('login')
		expect(port.send.mock.calls.map(([event]) => event)).toEqual([
			{
				name: 'page_view',
				page: { page_path: '/mypage', page_title: 'マイページ' },
			},
			{ name: 'login' },
		])
		expect([...values]).toEqual([[consentKey, 'granted']])
		consent.set('denied')
		gate.event('login')
		gate.page('/mypage')
		expect(port.send).toHaveBeenCalledTimes(2)
		expect(port.stop).toHaveBeenCalledTimes(1)
		expect([...values]).toEqual([[consentKey, 'denied']])
		dispose()
	})
	it('restores only non-identifying choices and fails closed for invalid/unavailable storage', () => {
		const { values, storage } = storageFixture()
		values.set(consentKey, '{"uid":"synthetic"}')
		expect(new ConsentStore(storage).getSnapshot().value).toBe('unset')
		values.set(consentKey, 'denied')
		expect(new ConsentStore(storage).getSnapshot().value).toBe('denied')
		const broken: PreferenceStorage = {
			getItem: () => {
				throw new Error('private detail')
			},
			setItem: () => {
				throw new Error('private detail')
			},
			removeItem: () => {
				throw new Error('private detail')
			},
		}
		const consent = new ConsentStore(broken)
		expect(consent.getSnapshot().value).toBe('unset')
		consent.set('denied')
		expect(consent.getSnapshot()).toEqual({ value: 'denied', saved: false })
	})
	it('cross-tab withdrawal stops future sends and malformed page data cannot escape', () => {
		const { values, storage } = storageFixture()
		const consent = new ConsentStore(storage)
		const port = { start: vi.fn(), stop: vi.fn(), send: vi.fn() }
		const gate = new AnalyticsGate(consent, port)
		const dispose = gate.mount()
		consent.set('granted')
		values.set(consentKey, 'denied')
		consent.reload()
		gate.event('login')
		expect(port.stop).toHaveBeenCalledTimes(1)
		expect(port.send).not.toHaveBeenCalled()
		dispose()
		expect(
			safePage(
				'/api/auth/youtube/callback?code=synthetic-secret&state=synthetic',
			),
		).toBeNull()
		expect(safePage('https://external.invalid/mypage')).toBeNull()
		expect(safePage('/not-allowed?uid=synthetic')).toBeNull()
		expect(
			safePage('/login?supportChallenge=synthetic&error=private#private'),
		).toEqual({ page_path: '/login', page_title: 'ログイン' })
	})
	it('retains only fixed complete UTM pairs and strips mixed/duplicated arbitrary values', () => {
		expect(
			safePage(
				'/?utm_source=youtube&utm_campaign=mypage&utm_medium=live_chat&utm_content=bot_command&code=synthetic#private',
			)?.page_path,
		).toBe(
			'/?utm_source=youtube&utm_medium=live_chat&utm_campaign=mypage&utm_content=bot_command',
		)
		expect(
			safePage(
				'/?utm_source=youtube&utm_campaign=mypage&utm_medium=live_chat&utm_content=synthetic-name',
			)?.page_path,
		).toBe('/')
		expect(
			safePage(
				'/?utm_source=youtube&utm_source=synthetic&utm_campaign=mypage&utm_medium=live_chat&utm_content=bot_command',
			)?.page_path,
		).toBe('/')
	})
})
