import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { MyPage } from './contract'
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
	beforeEach(() => vi.useFakeTimers())
	afterEach(() => vi.useRealTimers())

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

	it('preserves signout failure after auth rejection and can retry to recover', async () => {
		const load = vi
			.fn()
			.mockRejectedValue(new RequestError(401, 'AUTH_REQUIRED'))
		const signOut = vi
			.fn()
			.mockRejectedValueOnce(new Error('synthetic signout failure'))
			.mockResolvedValueOnce(undefined)
		const memory = new MyPageMemory(load, signOut)
		memory.setIdentity('sample')
		await settle()
		expect(memory.getSnapshot().phase).toBe('anonymous')
		expect(memory.getSnapshot().error).toBe('LOGOUT_FAILED')
		expect(memory.getSnapshot().data).toBeNull()
		memory.setIdentity('sample')
		await settle()
		expect(load).toHaveBeenCalledTimes(2)
		await memory.logout()
		expect(memory.getSnapshot().error).toBeNull()
		expect(signOut).toHaveBeenCalledTimes(2)
		load.mockResolvedValue(sample())
		memory.setIdentity('sample')
		await settle()
		expect(memory.getSnapshot().phase).toBe('authenticated')
		expect(memory.getSnapshot().data).not.toBeNull()
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

	it('polls 60s after completion, stops while hidden, refreshes on return without overlap', async () => {
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
		await vi.advanceTimersByTimeAsync(59_999)
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
