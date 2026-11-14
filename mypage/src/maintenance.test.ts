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

	it('accepts exactly one boolean and rejects malformed status responses', () => {
		for (const maintenanceEnabled of [true, false])
			expect(parseMaintenanceStatus({ maintenanceEnabled })).toEqual({
				maintenanceEnabled,
			})
		for (const invalid of [
			null,
			{},
			{ maintenanceEnabled: 'false' },
			{ maintenanceEnabled: null },
			{ maintenanceEnabled: false, updatedAt: '2026-10-27T02:00:00Z' },
			{
				state: 'NORMAL',
				updatedAt: '2026-10-27T02:00:00Z',
				messageTemplate: 'MYPAGE_MAINTENANCE_V1',
			},
		])
			expect(() => parseMaintenanceStatus(invalid)).toThrow()
	})

	it('checks on entry and recovers only after an explicit recheck', async () => {
		vi.useFakeTimers()
		const { documentTarget, windowTarget } = browserFixture()
		let maintenanceEnabled = true
		const request = vi.fn<typeof fetch>(async () =>
			Response.json({ maintenanceEnabled }),
		)
		const monitor = new MaintenanceMonitor(request)
		const dispose = monitor.mount()
		monitor.setRouteActive(true)
		await settle()
		expect(monitor.getSnapshot().phase).toBe('active')
		maintenanceEnabled = false
		await vi.advanceTimersByTimeAsync(360_000)
		documentTarget.dispatchEvent(new Event('visibilitychange'))
		windowTarget.dispatchEvent(
			Object.assign(new Event('pageshow'), { persisted: true }),
		)
		await settle()
		expect(request).toHaveBeenCalledTimes(1)
		expect(monitor.getSnapshot().phase).toBe('active')
		await monitor.check()
		expect(request).toHaveBeenCalledTimes(2)
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
})
