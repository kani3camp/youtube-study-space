import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import availableWire from '../../../../docs/mypage/fixtures/available.json'
import partialWire from '../../../../docs/mypage/fixtures/partial.json'
import type { Metric, MyPage } from './contract'
import { MyPageMemory, mergeResponse, RequestError } from './memory'

function sample(): MyPage {
	const metric = {
		availability: 'available',
		workSec: 3600,
		reasonCode: null,
	} as const
	return {
		generatedAt: '2026-10-06T03:00:00Z',
		timezone: 'Asia/Tokyo',
		partial: false,
		current: {
			availability: 'available',
			reasonCode: null,
			data: {
				state: 'work',
				workName: '合成作業',
				roomType: 'standard',
				seatNumber: 12,
				stateStartedAt: '2026-10-06T02:00:00Z',
				expectedEndAt: '2026-10-06T04:00:00Z',
			},
		},
		account: {
			availability: 'available',
			reasonCode: null,
			data: { displayName: 'Sample', handle: null, avatarUrl: null },
		},
		summary: {
			availability: 'available',
			reasonCode: null,
			data: { today: metric, week: metric, lifetime: metric },
		},
		recent7Days: {
			availability: 'available',
			reasonCode: null,
			data: Array.from({ length: 7 }, (_, index) => ({
				date: `2026-10-0${index + 1}`,
				...metric,
			})),
		},
	}
}

function deferred<T>() {
	let resolve: (value: T) => void = () => {
		throw new Error('not initialized')
	}
	let reject: (error: unknown) => void = () => {
		throw new Error('not initialized')
	}
	const promise = new Promise<T>((yes, no) => {
		resolve = yes
		reject = no
	})
	return { promise, resolve, reject }
}

async function settle() {
	for (let i = 0; i < 8; i++) await Promise.resolve()
}

describe('private MyPage memory', () => {
	beforeEach(() => {
		vi.useFakeTimers()
		// 合成データのgeneratedAtと同じJST日に固定する。
		vi.setSystemTime(new Date('2026-10-06T03:00:00Z'))
	})
	afterEach(() => vi.useRealTimers())
	it('refreshes only current state on the timer and refreshes totals manually', async () => {
		const next = sample()
		next.generatedAt = '2026-10-06T03:05:00Z'
		const load = vi
			.fn()
			.mockResolvedValueOnce(sample())
			.mockResolvedValueOnce(next)
			.mockResolvedValueOnce(next)
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('synthetic')
		await settle()
		const before = memory.getSnapshot().data
		await vi.advanceTimersByTimeAsync(5 * 60_000)
		expect(load.mock.calls.map((call) => call[4])).toEqual([false, true])
		expect(memory.getSnapshot().data?.current.asOf).toBe(next.generatedAt)
		expect(memory.getSnapshot().data?.summary).toBe(before?.summary)
		expect(memory.getSnapshot().data?.recent).toBe(before?.recent)
		await memory.refresh(true)
		expect(load.mock.calls[2]?.[4]).toBe(false)
		expect(memory.getSnapshot().data?.summary.today.asOf).toBe(next.generatedAt)
		memory.dispose()
	})

	describe('JST日付をまたいだ定期取得', () => {
		function at(generatedAt: string): MyPage {
			const page = sample()
			page.generatedAt = generatedAt
			return page
		}
		const loadedFlags = (load: { mock: { calls: unknown[][] } }) =>
			load.mock.calls.map((call) => call[4])

		it('同じJST日の定期取得はcurrentOnlyのままにする', async () => {
			vi.setSystemTime(new Date('2026-10-06T03:00:00Z'))
			const load = vi
				.fn()
				.mockResolvedValueOnce(at('2026-10-06T03:00:00Z'))
				.mockResolvedValue(at('2026-10-06T03:05:00Z'))
			const memory = new MyPageMemory(load, async () => {})
			memory.setIdentity('synthetic')
			await settle()
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(loadedFlags(load)).toEqual([false, true, true])
			memory.dispose()
		})

		it('日付が変わった最初の定期取得で全項目を取得し、その後はcurrentOnlyに戻る', async () => {
			vi.setSystemTime(new Date('2026-10-06T14:56:00Z'))
			const rolled = at('2026-10-06T15:01:00Z')
			rolled.recent7Days.data = rolled.recent7Days.data.map((item, index) => ({
				...item,
				date: `2026-10-0${index + 2}`,
			}))
			const load = vi
				.fn()
				.mockResolvedValueOnce(at('2026-10-06T14:56:00Z'))
				.mockResolvedValueOnce(rolled)
				.mockResolvedValue(at('2026-10-06T15:06:00Z'))
			const memory = new MyPageMemory(load, async () => {})
			memory.setIdentity('synthetic')
			await settle()
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(loadedFlags(load)).toEqual([false, false])
			const data = memory.getSnapshot().data
			expect(data?.generatedAt).toBe('2026-10-06T15:01:00Z')
			expect(data?.summary.today.asOf).toBe('2026-10-06T15:01:00Z')
			expect(data?.recent.at(-1)?.date).toBe('2026-10-08')
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(loadedFlags(load)).toEqual([false, false, true])
			memory.dispose()
		})

		it('日付をまたぐ全項目の取得に失敗したら、次の定期取得でも全項目を再試行する', async () => {
			vi.setSystemTime(new Date('2026-10-06T14:56:00Z'))
			const load = vi
				.fn()
				.mockResolvedValueOnce(at('2026-10-06T14:56:00Z'))
				.mockRejectedValueOnce(new RequestError(503, 'TEMPORARY_UNAVAILABLE'))
				.mockResolvedValueOnce(at('2026-10-06T15:10:00Z'))
				.mockResolvedValue(at('2026-10-06T15:15:00Z'))
			const memory = new MyPageMemory(load, async () => {})
			memory.setIdentity('synthetic')
			await settle()
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(memory.getSnapshot().error).toBe('TEMPORARY_UNAVAILABLE')
			expect(memory.getSnapshot().data?.generatedAt).toBe(
				'2026-10-06T14:56:00Z',
			)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(loadedFlags(load)).toEqual([false, false, false])
			expect(memory.getSnapshot().data?.generatedAt).toBe(
				'2026-10-06T15:10:00Z',
			)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(loadedFlags(load)).toEqual([false, false, false, true])
			memory.dispose()
		})
	})

	it('日付をまたぐ全項目取得が部分応答でも、集計が取れるまで全項目取得を続ける', async () => {
		vi.setSystemTime(new Date('2026-10-06T14:56:00Z'))
		const partial = sample()
		partial.generatedAt = '2026-10-06T15:01:00Z'
		partial.partial = true
		partial.summary = {
			availability: 'unavailable',
			reasonCode: 'SOURCE_UNAVAILABLE',
			data: null,
		} as unknown as MyPage['summary']
		partial.recent7Days = {
			availability: 'unavailable',
			reasonCode: 'SOURCE_UNAVAILABLE',
			data: [],
		} as unknown as MyPage['recent7Days']
		const first = sample()
		first.generatedAt = '2026-10-06T14:56:00Z'
		const recovered = sample()
		recovered.generatedAt = '2026-10-06T15:11:00Z'
		const load = vi
			.fn()
			.mockResolvedValueOnce(first)
			.mockResolvedValueOnce(partial)
			.mockResolvedValueOnce(recovered)
			.mockResolvedValue(recovered)
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('synthetic')
		await settle()
		await vi.advanceTimersByTimeAsync(5 * 60_000)
		await vi.advanceTimersByTimeAsync(5 * 60_000)
		expect(load.mock.calls.map((call) => call[4])).toEqual([
			false,
			false,
			false,
		])
		expect(memory.getSnapshot().data?.summary.today.asOf).toBe(
			'2026-10-06T15:11:00Z',
		)
		await vi.advanceTimersByTimeAsync(5 * 60_000)
		expect(load.mock.calls.map((call) => call[4])).toEqual([
			false,
			false,
			false,
			true,
		])
		memory.dispose()
	})

	describe('日付同期済みの判定', () => {
		function historyDown(generatedAt: string): MyPage {
			const page = sample()
			page.generatedAt = generatedAt
			page.partial = true
			const down = {
				availability: 'unavailable',
				reasonCode: 'SOURCE_UNAVAILABLE',
			} as unknown as Metric
			if (page.summary.data) {
				page.summary.data = { ...page.summary.data, today: down, week: down }
			}
			page.recent7Days = {
				availability: 'unavailable',
				reasonCode: 'SOURCE_UNAVAILABLE',
				data: [],
			} as unknown as MyPage['recent7Days']
			return page
		}
		function full(generatedAt: string): MyPage {
			const page = sample()
			page.generatedAt = generatedAt
			return page
		}
		const flags = (load: { mock: { calls: unknown[][] } }) =>
			load.mock.calls.map((call) => call[4])

		it('初回が部分応答で、日付をまたぐ取得でも履歴が失敗する間は全項目取得を続ける', async () => {
			vi.setSystemTime(new Date('2026-10-06T14:56:00Z'))
			const load = vi
				.fn()
				.mockResolvedValueOnce(historyDown('2026-10-06T14:56:00Z'))
				.mockResolvedValueOnce(historyDown('2026-10-06T15:01:00Z'))
				.mockResolvedValueOnce(historyDown('2026-10-06T15:06:00Z'))
				.mockResolvedValueOnce(full('2026-10-06T15:11:00Z'))
				.mockResolvedValue(full('2026-10-06T15:16:00Z'))
			const memory = new MyPageMemory(load, async () => {})
			memory.setIdentity('synthetic')
			await settle()
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(flags(load)).toEqual([false, false, false, false])
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(flags(load)).toEqual([false, false, false, false, true])
			memory.dispose()
		})

		it('同じ日の初回が部分応答でも、その日の定期取得はcurrentOnlyのままにする', async () => {
			const load = vi
				.fn()
				.mockResolvedValueOnce(historyDown('2026-10-06T03:00:00Z'))
				.mockResolvedValue(full('2026-10-06T03:05:00Z'))
			const memory = new MyPageMemory(load, async () => {})
			memory.setIdentity('synthetic')
			await settle()
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(flags(load)).toEqual([false, true, true])
			memory.dispose()
		})

		it('日付をまたいだ後の手動更新は、部分応答でもその日の同期試行として扱う', async () => {
			vi.setSystemTime(new Date('2026-10-06T14:56:00Z'))
			const load = vi
				.fn()
				.mockResolvedValueOnce(full('2026-10-06T14:56:00Z'))
				.mockResolvedValueOnce(historyDown('2026-10-06T15:01:00Z'))
				.mockResolvedValue(full('2026-10-06T15:06:00Z'))
			const memory = new MyPageMemory(load, async () => {})
			memory.setIdentity('synthetic')
			await settle()
			await vi.advanceTimersByTimeAsync(4 * 60_000 + 30_000)
			await memory.refresh(true)
			await vi.advanceTimersByTimeAsync(5 * 60_000)
			expect(flags(load)).toEqual([false, false, true])
			memory.dispose()
		})
	})

	it('restores totals after a successful token retry during polling', async () => {
		const retry = deferred<MyPage>()
		const next = sample()
		next.generatedAt = '2026-10-06T03:05:00Z'
		const load = vi
			.fn()
			.mockResolvedValueOnce(sample())
			.mockRejectedValueOnce(new RequestError(401, 'AUTH_REQUIRED'))
			.mockReturnValueOnce(retry.promise)
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('synthetic')
		await settle()
		await vi.advanceTimersByTimeAsync(5 * 60_000)
		expect(memory.getSnapshot().data).toBeNull()
		expect(load.mock.calls.map((call) => [call[2], call[4]])).toEqual([
			[false, false],
			[false, true],
			[true, false],
		])
		retry.resolve(next)
		await settle()
		expect(memory.getSnapshot().data?.summary.today.asOf).toBe(next.generatedAt)
		memory.dispose()
	})

	it('keeps last successful values with their timestamps when the history read fails', () => {
		const failed = structuredClone(partialWire) as MyPage
		failed.generatedAt = '2026-10-06T03:05:00Z'
		const first = mergeResponse(null, failed)
		expect(first.recent[6]).toMatchObject({
			data: null,
			asOf: null,
			stale: true,
			reason: 'HISTORY_LIMIT_EXCEEDED',
		})
		const prior = mergeResponse(null, availableWire as MyPage)
		const retained = mergeResponse(prior, failed)
		expect(retained.recent[6]).toMatchObject({
			data: prior.recent[6].data,
			asOf: prior.recent[6].asOf,
			stale: true,
			reason: 'HISTORY_LIMIT_EXCEEDED',
		})
	})

	it('drops previously displayed account metadata when the server reports expiration', () => {
		const first = mergeResponse(null, sample())
		const expired = sample()
		expired.account = {
			availability: 'unavailable',
			reasonCode: 'METADATA_TOO_OLD',
			data: null,
		}
		const result = mergeResponse(first, expired)
		expect(result.account).toEqual({
			data: null,
			asOf: null,
			stale: true,
			reason: 'METADATA_TOO_OLD',
		})
		expect(result.current.data?.workName).toBe('合成作業')
	})
	it('clears private memory and rejects a late response when maintenance activates', async () => {
		const late = deferred<MyPage>()
		const onMaintenance = vi.fn()
		const memory = new MyPageMemory(
			vi.fn().mockResolvedValueOnce(sample()).mockReturnValueOnce(late.promise),
			async () => {},
			Date.now,
			onMaintenance,
		)
		memory.setIdentity('synthetic')
		await settle()
		expect(memory.getSnapshot().data?.account.data?.displayName).toBe('Sample')
		void memory.refresh(true)
		await settle()
		memory.enterMaintenance()
		expect(memory.getSnapshot()).toEqual({
			phase: 'maintenance',
			data: null,
			busy: false,
			error: 'MAINTENANCE_ACTIVE',
			receivedAt: null,
		})
		expect(onMaintenance).toHaveBeenCalledTimes(1)
		late.resolve(sample())
		await settle()
		expect(memory.getSnapshot().data).toBeNull()
		memory.resumeAfterMaintenance()
		expect(memory.getSnapshot().phase).toBe('authenticated')
		memory.dispose()
	})
	it.each([
		'SERVICE_ACCESS_RESTRICTED',
		'DATA_DELETION_IN_PROGRESS',
	])('clears all private values and stops retry, polling and restored same-uid access after %s', async (code) => {
		const load = vi
			.fn()
			.mockResolvedValueOnce(sample())
			.mockRejectedValue(new RequestError(403, code))
		const signOut = vi.fn(async () => {})
		const memory = new MyPageMemory(load, signOut)
		memory.setIdentity('synthetic')
		await settle()
		expect(memory.getSnapshot().data?.current.data?.workName).toBe('合成作業')
		await memory.refresh()
		expect(memory.getSnapshot()).toEqual({
			phase: 'restricted',
			data: null,
			busy: false,
			error: code,
			receivedAt: null,
		})
		expect(load.mock.calls[1]?.[1].aborted).toBe(true)
		await memory.refresh(true)
		memory.setVisible(false)
		memory.setVisible(true)
		memory.suspend()
		memory.setIdentity('synthetic')
		await vi.advanceTimersByTimeAsync(600_000)
		expect(load).toHaveBeenCalledTimes(2)
		expect(signOut).not.toHaveBeenCalled()
		expect(memory.getSnapshot().phase).toBe('restricted')
		await memory.logout()
		expect(signOut).toHaveBeenCalledTimes(1)
		expect(memory.getSnapshot().phase).toBe('anonymous')
		memory.dispose()
	})
	it('honors Retry-After across manual refresh and hide/show, then resumes at the deadline', async () => {
		const load = vi
			.fn()
			.mockRejectedValueOnce(new RequestError(429, 'RATE_LIMITED', 120))
			.mockResolvedValue(sample())
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('sample')
		await settle()
		await memory.refresh(true)
		memory.setVisible(false)
		memory.setVisible(true)
		await settle()
		await vi.advanceTimersByTimeAsync(119_999)
		await memory.refresh(true)
		expect(load).toHaveBeenCalledTimes(1)
		memory.setVisible(false)
		await vi.advanceTimersByTimeAsync(1)
		expect(load).toHaveBeenCalledTimes(1)
		memory.setVisible(true)
		await settle()
		expect(load).toHaveBeenCalledTimes(2)
		memory.dispose()
	})

	it('discards an old uid response even if abort is ignored', async () => {
		const old = deferred<MyPage>()
		const next = deferred<MyPage>()
		const signals: AbortSignal[] = []
		const memory = new MyPageMemory(
			vi.fn((uid, signal) => {
				signals.push(signal)
				return uid === 'old' ? old.promise : next.promise
			}),
			async () => {},
		)
		expect(memory.getSnapshot().phase).toBe('bootstrapping')
		memory.setIdentity('old')
		memory.setIdentity('new')
		expect(signals[0]?.aborted).toBe(true)
		old.resolve(sample())
		await settle()
		expect(memory.getSnapshot().data).toBeNull()
		const data = sample()
		data.account = {
			availability: 'available',
			reasonCode: null,
			data: { displayName: 'New sample', handle: null, avatarUrl: null },
		}
		next.resolve(data)
		await settle()
		expect(memory.getSnapshot().data?.account.data?.displayName).toBe(
			'New sample',
		)
		memory.dispose()
	})

	it('clears synchronously before logout, retains no data on failed signout, permits retry', async () => {
		const signedOut = deferred<void>()
		const signOut = vi
			.fn()
			.mockReturnValueOnce(signedOut.promise)
			.mockResolvedValueOnce(undefined)
		const memory = new MyPageMemory(async () => sample(), signOut)
		memory.setIdentity('sample')
		await settle()
		expect(memory.getSnapshot().data).not.toBeNull()
		const logout = memory.logout()
		expect(memory.getSnapshot().data).toBeNull()
		expect(memory.getSnapshot().phase).toBe('signing-out')
		memory.setIdentity('sample')
		signedOut.reject(new Error('synthetic failure'))
		await logout
		expect(memory.getSnapshot().error).toBe('LOGOUT_FAILED')
		expect(memory.getSnapshot().data).toBeNull()
		await memory.logout()
		expect(signOut).toHaveBeenCalledTimes(2)
		memory.dispose()
	})

	it('does not let an old logout completion overwrite a new authenticated session', async () => {
		const signedOut = deferred<void>()
		const memory = new MyPageMemory(
			async () => sample(),
			() => signedOut.promise,
		)
		memory.setIdentity('old')
		await settle()
		const logout = memory.logout()
		memory.setIdentity(null)
		memory.setIdentity('new')
		await settle()
		signedOut.resolve()
		await logout
		expect(memory.getSnapshot().phase).toBe('authenticated')
		expect(memory.getSnapshot().data).not.toBeNull()
		memory.dispose()
	})

	it('polls 5m after completion, stops while hidden, refreshes on return without overlap', async () => {
		const pending = deferred<MyPage>()
		const load = vi
			.fn()
			.mockReturnValueOnce(pending.promise)
			.mockResolvedValue(sample())
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('sample')
		await vi.advanceTimersByTimeAsync(120_000)
		expect(load).toHaveBeenCalledTimes(1)
		pending.resolve(sample())
		await settle()
		await vi.advanceTimersByTimeAsync(299_999)
		expect(load).toHaveBeenCalledTimes(1)
		await vi.advanceTimersByTimeAsync(1)
		expect(load).toHaveBeenCalledTimes(2)
		memory.setVisible(false)
		await vi.advanceTimersByTimeAsync(300_000)
		expect(load).toHaveBeenCalledTimes(2)
		memory.setVisible(true)
		await settle()
		expect(load).toHaveBeenCalledTimes(3)
		memory.dispose()
	})

	it.each([
		'route',
		'offline',
	] as const)('fetches only while MyPage is visible and online: %s', async (boundary) => {
		const load = vi.fn().mockResolvedValue(sample())
		const memory = new MyPageMemory(load, async () => {})
		const enable = (active: boolean) =>
			boundary === 'route'
				? memory.setRouteActive(active)
				: memory.setOnline(active)
		enable(false)
		memory.setIdentity('sample')
		await settle()
		await vi.advanceTimersByTimeAsync(600_000)
		await memory.refresh(true)
		expect(load).not.toHaveBeenCalled()
		enable(true)
		await settle()
		expect(load).toHaveBeenCalledTimes(1)
		await vi.advanceTimersByTimeAsync(30_000)
		enable(false)
		enable(true)
		await vi.advanceTimersByTimeAsync(269_999)
		expect(load).toHaveBeenCalledTimes(1)
		await vi.advanceTimersByTimeAsync(1)
		expect(load).toHaveBeenCalledTimes(2)
		enable(false)
		memory.setVisible(false)
		await vi.advanceTimersByTimeAsync(600_000)
		enable(true)
		await settle()
		expect(load).toHaveBeenCalledTimes(2)
		memory.setVisible(true)
		await settle()
		expect(load).toHaveBeenCalledTimes(3)
		memory.dispose()
	})

	it('keeps transient backoff bounded across a visibility change', async () => {
		const load = vi
			.fn()
			.mockRejectedValue(new RequestError(503, 'TEMPORARY_UNAVAILABLE'))
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('sample')
		await settle()
		await vi.advanceTimersByTimeAsync(60_000)
		expect(load).toHaveBeenCalledTimes(2)
		memory.setVisible(false)
		memory.setVisible(true)
		await vi.advanceTimersByTimeAsync(119_999)
		expect(load).toHaveBeenCalledTimes(2)
		load.mockResolvedValue(sample())
		await vi.advanceTimersByTimeAsync(1)
		expect(load).toHaveBeenCalledTimes(3)
		expect(memory.getSnapshot().error).toBeNull()
		memory.dispose()
	})

	it('clears before a single forced-token retry and signs out on repeated 401', async () => {
		const force = deferred<MyPage>()
		const load = vi
			.fn()
			.mockResolvedValueOnce(sample())
			.mockRejectedValueOnce(new RequestError(401, 'AUTH_REQUIRED'))
			.mockReturnValueOnce(force.promise)
		const signOut = vi.fn(async () => {})
		const memory = new MyPageMemory(load, signOut)
		memory.setIdentity('sample')
		await settle()
		const refresh = memory.refresh()
		await settle()
		expect(memory.getSnapshot().data).toBeNull()
		expect(load.mock.calls[2]?.[2]).toBe(true)
		force.reject(new RequestError(401, 'AUTH_REQUIRED'))
		await refresh
		expect(load).toHaveBeenCalledTimes(3)
		expect(signOut).toHaveBeenCalledTimes(1)
		expect(memory.getSnapshot().phase).toBe('anonymous')
		memory.dispose()
	})

	it('bounds App Check retry and respects Retry-After', async () => {
		const load = vi
			.fn()
			.mockRejectedValueOnce(new RequestError(403, 'APP_CHECK_REQUIRED'))
			.mockRejectedValueOnce(new RequestError(429, 'RATE_LIMITED', 120))
			.mockResolvedValue(sample())
		const memory = new MyPageMemory(load, async () => {})
		memory.setIdentity('sample')
		await settle()
		expect(load.mock.calls[1]?.[3]).toBe(true)
		await vi.advanceTimersByTimeAsync(119_999)
		expect(load).toHaveBeenCalledTimes(2)
		await vi.advanceTimersByTimeAsync(1)
		expect(load).toHaveBeenCalledTimes(3)
		memory.suspend()
		expect(memory.getSnapshot().data).toBeNull()
		expect(memory.getSnapshot().phase).toBe('bootstrapping')
		memory.dispose()
	})

	it('retains previous successful metric with its original JST date, never substitutes zero', () => {
		const first = mergeResponse(null, sample())
		const next = sample()
		next.generatedAt = '2026-10-06T15:01:00Z'
		next.summary = {
			availability: 'unavailable',
			reasonCode: 'SOURCE_UNAVAILABLE',
			data: null,
		}
		next.current = {
			availability: 'unavailable',
			reasonCode: 'SOURCE_UNAVAILABLE',
			data: null,
		}
		const retained = mergeResponse(first, next)
		expect(retained.summary.today).toEqual({
			data: 3600,
			stale: true,
			reason: 'SOURCE_UNAVAILABLE',
			asOf: first.generatedAt,
		})
		expect(retained.current.data?.workName).toBe('合成作業')
		expect(mergeResponse(null, next).summary.today.data).toBeNull()
	})
})
