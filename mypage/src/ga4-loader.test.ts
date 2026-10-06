import { describe, expect, it, vi } from 'vitest'
import type { GtagCommand } from './ga4'
import {
	browserTagLoader,
	ConsentTag,
	type PreparedTag,
	type TagLoader,
	type TagWindow,
} from './ga4-loader'
import { AnalyticsGate, ConsentStore } from './privacy'

function at<T>(array: T[], index: number): T {
	const value = array[index]
	if (value === undefined) throw new Error('Missing synthetic fixture')
	return value
}

function deferred() {
	let resolve: () => void = () => {}
	let reject: (reason: Error) => void = () => {}
	const promise = new Promise<void>((a, b) => {
		resolve = a
		reject = b
	})
	return { promise, resolve, reject }
}
function fixture() {
	const loads: {
		ready: ReturnType<typeof deferred>
		command: ReturnType<typeof vi.fn>
		cancel: ReturnType<typeof vi.fn>
		disable: ReturnType<typeof vi.fn>
	}[] = []
	const loader: TagLoader = (prepare) => {
		const ready = deferred()
		const command = vi.fn<(...args: GtagCommand) => void>()
		const cancel = vi.fn()
		const disable = vi.fn()
		loads.push({ ready, command, cancel, disable })
		const runtime = { command, disable }
		prepare(runtime)
		return { runtime, ready: ready.promise, cancel }
	}
	const consent = new ConsentStore(null)
	const tag = new ConsentTag(
		consent,
		'G-SYNTHETIC1',
		'https://example.invalid',
		loader,
	)
	const gate = new AnalyticsGate(consent, tag)
	return { loads, consent, gate }
}

describe('consent-bound asynchronous loading', () => {
	it('drops events before consent and during loading, emits only future sanitized events', async () => {
		const { loads, consent, gate } = fixture()
		const dispose = gate.mount()
		gate.page('/login?code=synthetic-private')
		consent.set('denied')
		expect(loads).toHaveLength(0)
		consent.set('granted')
		expect(loads).toHaveLength(1)
		const first = at(loads, 0)
		gate.page('/mypage?uid=synthetic-private')
		gate.event('login')
		expect(first.command.mock.calls.map(([kind]) => kind)).toEqual([
			'consent',
			'config',
		])
		first.ready.resolve()
		await Promise.resolve()
		expect(first.command).toHaveBeenCalledTimes(2)
		gate.page('/mypage?uid=synthetic-private')
		gate.event('login')
		expect(first.command.mock.calls.map(([kind]) => kind)).toEqual([
			'consent',
			'config',
			'event',
			'event',
		])
		expect(JSON.stringify(first.command.mock.calls)).not.toContain(
			'synthetic-private',
		)
		consent.set('denied')
		gate.event('login')
		expect(first.disable).toHaveBeenLastCalledWith('G-SYNTHETIC1', true)
		expect(first.cancel).toHaveBeenCalledOnce()
		expect(first.command).toHaveBeenCalledTimes(4)
		dispose()
	})
	it('ignores an old successful or failed load after withdrawal and regrant', async () => {
		for (const fail of [false, true]) {
			const { loads, consent, gate } = fixture()
			const dispose = gate.mount()
			consent.set('granted')
			consent.set('denied')
			consent.set('granted')
			const old = at(loads, 0)
			const current = at(loads, 1)
			current.ready.resolve()
			await Promise.resolve()
			if (fail) old.ready.reject(new Error('synthetic'))
			else old.ready.resolve()
			await Promise.resolve()
			gate.event('login')
			expect(old.command).toHaveBeenCalledTimes(2)
			expect(current.command.mock.calls.at(-1)?.slice(0, 2)).toEqual([
				'event',
				'login',
			])
			expect(current.cancel).not.toHaveBeenCalled()
			dispose()
		}
	})
	it('failed load/unmount never starts sending and has no replay', async () => {
		const { loads, consent, gate } = fixture()
		const dispose = gate.mount()
		consent.set('granted')
		at(loads, 0).ready.reject(new Error('synthetic'))
		await Promise.resolve()
		gate.event('login')
		expect(at(loads, 0).command).toHaveBeenCalledTimes(2)
		consent.set('denied')
		consent.set('granted')
		dispose()
		at(loads, 1).ready.resolve()
		await Promise.resolve()
		gate.event('login')
		expect(at(loads, 1).command).toHaveBeenCalledTimes(2)
	})
})

function domFixture() {
	const scripts: {
		src: string
		async: boolean
		referrerPolicy: string
		onload: (() => void) | null
		onerror: (() => void) | null
		remove: ReturnType<typeof vi.fn>
	}[] = []
	const append = vi.fn((script) => {
		scripts.push(script)
	})
	const target: TagWindow = {}
	const doc = {
		createElement: () => ({ remove: vi.fn() }),
		head: { append },
	} as unknown as Document
	const loader = browserTagLoader('G-SYNTHETIC1', target, doc)
	function load() {
		const tag = loader((runtime) => {
			runtime.disable('G-SYNTHETIC1', false)
			runtime.command('consent', 'default', { analytics_storage: 'granted' })
			runtime.command('config', 'G-SYNTHETIC1', {
				send_page_view: false,
				page_location: 'https://example.invalid/',
			})
		})
		void tag.ready.catch(() => {})
		return tag
	}
	return { scripts, target, load }
}

describe('browser DOM boundary without vendor networking', () => {
	it('prepares safe startup commands before appending a fixed no-referrer script; cancellation clears queue', async () => {
		const { scripts, target, load } = domFixture()
		expect(target.ossAnalyticsLayer).toBeUndefined()
		const first = load()
		const old = at(scripts, 0).onload
		expect(scripts[0]).toMatchObject({
			src: 'https://www.googletagmanager.com/gtag/js?id=G-SYNTHETIC1&l=ossAnalyticsLayer',
			referrerPolicy: 'no-referrer',
			async: true,
		})
		expect(
			Array.from(target.ossAnalyticsLayer as unknown as IArguments[]).map(
				(x) => x[0],
			),
		).toEqual(['consent', 'js', 'config'])
		first.cancel()
		expect(target['ga-disable-G-SYNTHETIC1']).toBe(true)
		expect(target.ossAnalyticsLayer).toHaveLength(0)
		const second: PreparedTag = load()
		old?.()
		expect(target['ga-disable-G-SYNTHETIC1']).toBe(false)
		at(scripts, 1).onload?.()
		await second.ready
		first.cancel()
		expect(target['ga-disable-G-SYNTHETIC1']).toBe(false)
		second.cancel()
	})
	it('timeout disables and clears pending startup, and refuses shared tag ownership', async () => {
		vi.useFakeTimers()
		try {
			const { target, load } = domFixture()
			const tag = load()
			const rejected = expect(tag.ready).rejects.toThrow(
				'Analytics unavailable',
			)
			await vi.advanceTimersByTimeAsync(5000)
			await rejected
			expect(target['ga-disable-G-SYNTHETIC1']).toBe(true)
			expect(target.ossAnalyticsLayer).toHaveLength(0)
			expect(() =>
				browserTagLoader('G-SYNTHETIC1', target, {} as Document),
			).toThrow()
			expect(() => browserTagLoader('bad-id', {}, {} as Document)).toThrow()
		} finally {
			vi.useRealTimers()
		}
	})
})
