import { describe, expect, it, vi } from 'vitest'
import { GA4Sender, type GtagCommand } from './ga4'
import { AnalyticsGate, ConsentStore, type SafeEvent } from './privacy'

function fixture() {
	const command = vi.fn<(...args: GtagCommand) => void>()
	const disable = vi.fn()
	const sender = new GA4Sender('G-SYNTHETIC1', 'https://example.invalid', {
		command,
		disable,
	})
	const consent = new ConsentStore(null)
	const gate = new AnalyticsGate(consent, sender)
	return { command, disable, sender, consent, gate }
}

describe('GA4 mockable sender boundary (vendor not loaded)', () => {
	it('never initializes or queues pre-consent events, then opts out of automatic config pageviews', () => {
		const { gate, command, disable, consent } = fixture()
		const dispose = gate.mount()
		gate.page('/login?code=synthetic-private')
		gate.event('login')
		expect(command).not.toHaveBeenCalled()
		expect(disable).not.toHaveBeenCalled()
		consent.set('granted')
		expect(command.mock.calls.map(([kind]) => kind)).toEqual([
			'consent',
			'config',
		])
		expect(command.mock.calls[1]).toEqual([
			'config',
			'G-SYNTHETIC1',
			{
				send_page_view: false,
				allow_google_signals: false,
				allow_ad_personalization_signals: false,
				page_location: 'https://example.invalid/',
				page_referrer: '',
				page_title: '',
			},
		])
		gate.page('/mypage?uid=synthetic-private#synthetic-private')
		gate.event('login')
		expect(command.mock.calls[2]).toEqual([
			'event',
			'page_view',
			{
				send_to: 'G-SYNTHETIC1',
				page_location: 'https://example.invalid/mypage',
				page_title: 'マイページ',
				page_referrer: '',
				allow_google_signals: false,
				allow_ad_personalization_signals: false,
			},
		])
		expect(command.mock.calls[3]?.[1]).toBe('login')
		expect(JSON.stringify(command.mock.calls)).not.toContain(
			'synthetic-private',
		)
		expect(JSON.stringify(command.mock.calls)).not.toContain('user_id')
		dispose()
	})
	it('withdrawal disables collection before future sends and does not issue a denied ping or replay on regrant', () => {
		const { gate, command, disable, consent } = fixture()
		const dispose = gate.mount()
		consent.set('granted')
		gate.page('/mypage')
		const before = command.mock.calls.length
		consent.set('denied')
		gate.page('/login')
		gate.event('login')
		expect(disable).toHaveBeenLastCalledWith('G-SYNTHETIC1', true)
		expect(command).toHaveBeenCalledTimes(before)
		consent.set('granted')
		expect(command).toHaveBeenCalledTimes(before + 2)
		gate.event('login')
		expect(command.mock.calls.at(-1)?.[2]).toMatchObject({
			page_location: 'https://example.invalid/',
			page_title: '',
			page_referrer: '',
		})
		dispose()
	})
	it('revalidates transport input and cannot accept raw titles, IDs, params, callback routes or unsupported events', () => {
		const { sender, command } = fixture()
		sender.start()
		sender.send({
			name: 'page_view',
			page: {
				page_path: '/login?code=synthetic-private#secret',
				page_title: 'synthetic-private',
			},
			user_id: 'synthetic-private',
		} as SafeEvent)
		expect(command.mock.calls.at(-1)?.[2]).toMatchObject({
			page_location: 'https://example.invalid/login',
			page_title: 'ログイン',
			page_referrer: '',
		})
		const count = command.mock.calls.length
		sender.send({
			name: 'page_view',
			page: {
				page_path: '/api/auth/youtube/callback?code=synthetic-private',
				page_title: 'secret',
			},
		})
		sender.send({
			name: 'page_view',
			page: {
				page_path: 'https://external.invalid/mypage',
				page_title: 'secret',
			},
		})
		sender.send({ name: 'unsupported-secret' } as unknown as SafeEvent)
		expect(command).toHaveBeenCalledTimes(count)
		expect(JSON.stringify(command.mock.calls)).not.toContain(
			'synthetic-private',
		)
	})
	it('retains only fixed UTM tuples and stays off after vendor initialization fails', () => {
		const { sender, command } = fixture()
		sender.start()
		sender.send({
			name: 'page_view',
			page: {
				page_path:
					'/?utm_source=youtube&utm_campaign=mypage&utm_medium=live_chat&utm_content=bot_command&code=synthetic-private',
				page_title: 'private',
			},
		})
		expect(command.mock.calls.at(-1)?.[2]).toMatchObject({
			page_location:
				'https://example.invalid/?utm_source=youtube&utm_medium=live_chat&utm_campaign=mypage&utm_content=bot_command',
		})
		const broken = {
			command: vi.fn(() => {
				throw new Error('synthetic-private')
			}),
			disable: vi.fn(),
		}
		const failed = new GA4Sender(
			'G-SYNTHETIC1',
			'https://example.invalid',
			broken,
		)
		failed.start()
		failed.send({ name: 'login' })
		expect(broken.command).toHaveBeenCalledTimes(1)
		expect(broken.disable).toHaveBeenLastCalledWith('G-SYNTHETIC1', true)
	})
	it('rejects configuration that could smuggle identifiers or query data into every event', () => {
		const runtime = { command: vi.fn(), disable: vi.fn() }
		for (const origin of [
			'http://example.invalid',
			'https://example.invalid/path',
			'https://example.invalid?code=secret',
			'https://user:secret@example.invalid',
		]) {
			expect(() => new GA4Sender('G-SYNTHETIC1', origin, runtime)).toThrow()
		}
		expect(
			() => new GA4Sender('uid-secret', 'https://example.invalid', runtime),
		).toThrow()
		expect(runtime.command).not.toHaveBeenCalled()
	})
})
