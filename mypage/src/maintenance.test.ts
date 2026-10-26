import { afterEach, describe, expect, it, vi } from 'vitest'
import {
	MaintenanceMonitor,
	maintenanceDestination,
	parseMaintenanceStatus,
} from './maintenance'

async function settle() {
	for (let i = 0; i < 10; i++) await Promise.resolve()
}

function browserFixture() {
	const windowTarget = new EventTarget()
	const documentTarget = Object.assign(new EventTarget(), {
		visibilityState: 'visible',
	})
	vi.stubGlobal('window', windowTarget)
	vi.stubGlobal('document', documentTarget)
	return { windowTarget, documentTarget }
}

afterEach(() => {
	vi.useRealTimers()
	vi.unstubAllGlobals()
})

describe('public maintenance status', () => {
	it('keeps route recovery fail-safe while preserving purpose-bound support', () => {
		expect(maintenanceDestination('/maintenance', false, 'normal')).toBe('/')
		expect(maintenanceDestination('/mypage', false, 'active')).toBe(
			'/maintenance',
		)
		for (const phase of ['unknown', 'checking', 'normal'] as const)
			expect(maintenanceDestination('/mypage', false, phase)).toBeNull()
		expect(maintenanceDestination('/maintenance', false, 'active')).toBeNull()
		expect(maintenanceDestination('/maintenance', false, 'unknown')).toBeNull()
		expect(maintenanceDestination('/privacy', false, 'active')).toBeNull()
		expect(maintenanceDestination('/login', true, 'active')).toBeNull()
		expect(
			maintenanceDestination(
				'/login/channel-confirm',
				false,
				'active',
				'support',
			),
		).toBeNull()
		expect(
			maintenanceDestination(
				'/login/channel-confirm',
				false,
				'active',
				'login',
			),
		).toBe('/maintenance')
	})

	it('accepts only the minimal exact public schema', () => {
		const valid = {
			state: 'ACTIVE',
			updatedAt: '2026-10-27T02:00:00Z',
			estimatedEndAt: '2026-10-27T03:00:00Z',
			messageTemplate: 'MYPAGE_MAINTENANCE_V1',
		}
		expect(parseMaintenanceStatus(valid)).toEqual(valid)
		for (const invalid of [
			{ ...valid, state: 'NOTICE' },
			{ ...valid, revision: 2 },
			{ ...valid, reasonCategory: 'PLANNED_MAINTENANCE' },
			{ ...valid, messageTemplate: 'operator text' },
			{ ...valid, updatedAt: 'not-a-time' },
			{ ...valid, updatedAt: '2026-10-27T02:00:00' },
			{ ...valid, state: 'NORMAL' },
		])
			expect(() => parseMaintenanceStatus(invalid)).toThrow()
	})

	it('checks on entry and visible interval, stops hidden, and checks on return', async () => {
		vi.useFakeTimers()
		const { documentTarget } = browserFixture()
		const request = vi.fn<typeof fetch>(async () =>
			Response.json({
				state: 'ACTIVE',
				updatedAt: '2026-10-27T02:00:00Z',
				messageTemplate: 'MYPAGE_MAINTENANCE_V1',
			}),
		)
		const monitor = new MaintenanceMonitor(request)
		const dispose = monitor.mount()
		monitor.setRouteActive(true)
		await settle()
		expect(monitor.getSnapshot().phase).toBe('active')
		expect(request).toHaveBeenCalledTimes(1)
		await vi.advanceTimersByTimeAsync(120_000)
		expect(request).toHaveBeenCalledTimes(2)
		documentTarget.visibilityState = 'hidden'
		documentTarget.dispatchEvent(new Event('visibilitychange'))
		await vi.advanceTimersByTimeAsync(360_000)
		expect(request).toHaveBeenCalledTimes(2)
		documentTarget.visibilityState = 'visible'
		documentTarget.dispatchEvent(new Event('visibilitychange'))
		await settle()
		expect(request).toHaveBeenCalledTimes(3)
		dispose()
	})

	it('recovers ACTIVE to NORMAL on the next visible poll', async () => {
		vi.useFakeTimers()
		browserFixture()
		let state: 'ACTIVE' | 'NORMAL' = 'ACTIVE'
		const request = vi.fn<typeof fetch>(async () =>
			Response.json({
				state,
				updatedAt: '2026-10-27T02:00:00Z',
				messageTemplate: 'MYPAGE_MAINTENANCE_V1',
			}),
		)
		const monitor = new MaintenanceMonitor(request)
		const dispose = monitor.mount()
		monitor.setRouteActive(true)
		await settle()
		expect(monitor.getSnapshot().phase).toBe('active')
		state = 'NORMAL'
		await vi.advanceTimersByTimeAsync(120_000)
		expect(monitor.getSnapshot().phase).toBe('normal')
		dispose()
	})

	it('recovers ACTIVE to NORMAL when a hidden tab becomes visible', async () => {
		const { documentTarget } = browserFixture()
		let state: 'ACTIVE' | 'NORMAL' = 'ACTIVE'
		const monitor = new MaintenanceMonitor(async () =>
			Response.json({
				state,
				updatedAt: '2026-10-27T02:00:00Z',
				messageTemplate: 'MYPAGE_MAINTENANCE_V1',
			}),
		)
		const dispose = monitor.mount()
		monitor.setRouteActive(true)
		await settle()
		expect(monitor.getSnapshot().phase).toBe('active')
		documentTarget.visibilityState = 'hidden'
		documentTarget.dispatchEvent(new Event('visibilitychange'))
		state = 'NORMAL'
		documentTarget.visibilityState = 'visible'
		documentTarget.dispatchEvent(new Event('visibilitychange'))
		await settle()
		expect(monitor.getSnapshot().phase).toBe('normal')
		dispose()
	})

	it('keeps failures and malformed responses in UNKNOWN instead of guessing NORMAL', async () => {
		browserFixture()
		for (const response of [
			new Response(null, { status: 503 }),
			Response.json({ state: 'NORMAL' }),
		]) {
			const monitor = new MaintenanceMonitor(async () => response)
			const dispose = monitor.mount()
			monitor.setRouteActive(true)
			await settle()
			expect(monitor.getSnapshot()).toEqual({ phase: 'unknown', status: null })
			dispose()
		}
	})

	it('preserves the authoritative estimate when an API error repeats ACTIVE', async () => {
		browserFixture()
		const monitor = new MaintenanceMonitor(async () =>
			Response.json({
				state: 'ACTIVE',
				updatedAt: '2026-10-27T02:00:00Z',
				estimatedEndAt: '2026-10-27T03:00:00Z',
				messageTemplate: 'MYPAGE_MAINTENANCE_V1',
			}),
		)
		const dispose = monitor.mount()
		monitor.setRouteActive(true)
		await settle()
		monitor.markActive()
		expect(monitor.getSnapshot()).toMatchObject({
			phase: 'active',
			status: { estimatedEndAt: '2026-10-27T03:00:00Z' },
		})
		dispose()
	})

	it('closes the private gate synchronously for forced route and pageshow rechecks', async () => {
		const { windowTarget } = browserFixture()
		let release: (response: Response) => void = () => {}
		const phases: string[] = []
		const monitor = new MaintenanceMonitor(
			() =>
				new Promise<Response>((resolve) => {
					release = resolve
				}),
			(phase) => phases.push(phase),
		)
		const dispose = monitor.mount()
		monitor.setRouteActive(true)
		expect(monitor.getSnapshot().phase).toBe('checking')
		expect(phases.at(-1)).toBe('checking')
		release(
			Response.json({
				state: 'NORMAL',
				updatedAt: '2026-10-27T02:00:00Z',
				messageTemplate: 'MYPAGE_MAINTENANCE_V1',
			}),
		)
		await settle()
		expect(monitor.getSnapshot().phase).toBe('normal')
		monitor.setRouteActive(true, true)
		expect(monitor.getSnapshot().phase).toBe('checking')
		windowTarget.dispatchEvent(new Event('pageshow'))
		expect(monitor.getSnapshot().phase).toBe('checking')
		dispose()
	})
})
