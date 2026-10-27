import { checkedJSON, objectValue } from './features/mypage/client'

export type PublicMaintenanceStatus = {
	state: 'NORMAL' | 'ACTIVE'
	updatedAt: string
	messageTemplate: 'MYPAGE_MAINTENANCE_V1'
}

export type MaintenanceSnapshot =
	| { phase: 'checking' | 'unknown'; status: null }
	| { phase: 'normal' | 'active'; status: PublicMaintenanceStatus }

export function maintenanceDestination(
	pathname: string,
	supportLogin: boolean,
	phase: MaintenanceSnapshot['phase'],
	confirmationPurpose?: 'login' | 'support',
): '/' | '/maintenance' | null {
	if (pathname === '/maintenance') return phase === 'normal' ? '/' : null
	const protectedRoute =
		pathname.startsWith('/mypage') ||
		(pathname === '/login' && !supportLogin) ||
		(pathname === '/login/channel-confirm' && confirmationPurpose === 'login')
	if (!protectedRoute || phase !== 'active') return null
	return '/maintenance'
}

function timestamp(value: unknown) {
	if (typeof value !== 'string') return false
	const match =
		/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|([+-])(\d{2}):(\d{2}))$/.exec(
			value,
		)
	if (!match) return false
	const [, yearText, monthText, dayText, hourText, minuteText, secondText] =
		match
	const year = Number(yearText)
	const month = Number(monthText)
	const day = Number(dayText)
	const hour = Number(hourText)
	const minute = Number(minuteText)
	const second = Number(secondText)
	const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)
	const days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
	return (
		year >= 1 &&
		month >= 1 &&
		month <= 12 &&
		day >= 1 &&
		day <= days[month - 1] &&
		hour <= 23 &&
		minute <= 59 &&
		second <= 59 &&
		(match[8] === 'Z' ||
			(Number(match[10]) <= 23 && Number(match[11]) <= 59)) &&
		Number.isFinite(Date.parse(value))
	)
}

export function parseMaintenanceStatus(
	value: unknown,
): PublicMaintenanceStatus {
	const status = objectValue(value)
	if (!status) throw new Error('invalid maintenance status')
	const keys = Object.keys(status).sort().join(',')

	if (
		keys !== 'messageTemplate,state,updatedAt' ||
		(status.state !== 'NORMAL' && status.state !== 'ACTIVE') ||
		!timestamp(status.updatedAt) ||
		status.messageTemplate !== 'MYPAGE_MAINTENANCE_V1'
	)
		throw new Error('invalid maintenance status')
	return status as PublicMaintenanceStatus
}

export class MaintenanceMonitor {
	private state: MaintenanceSnapshot = { phase: 'checking', status: null }
	private listeners = new Set<() => void>()
	private controller: AbortController | null = null
	private mounted = false
	private routeActive = false
	private request: typeof fetch
	private closePrivateGate: (phase: 'checking' | 'unknown' | 'active') => void

	constructor(
		request: typeof fetch = fetch,
		closePrivateGate: (
			phase: 'checking' | 'unknown' | 'active',
		) => void = () => {},
	) {
		this.request = request
		this.closePrivateGate = closePrivateGate
	}

	getSnapshot = () => this.state
	allowsPrivateRestore = () =>
		!this.routeActive || this.state.phase === 'normal'
	subscribe = (listener: () => void) => {
		this.listeners.add(listener)
		return () => this.listeners.delete(listener)
	}

	private publish(state: MaintenanceSnapshot) {
		this.state = state
		if (state.phase !== 'normal') this.closePrivateGate(state.phase)
		for (const listener of this.listeners) listener()
	}

	markActive() {
		this.controller?.abort()
		if (this.state.phase !== 'active')
			this.publish({
				phase: 'active',
				status: {
					state: 'ACTIVE',
					updatedAt: new Date().toISOString(),
					messageTemplate: 'MYPAGE_MAINTENANCE_V1',
				},
			})
	}

	setRouteActive(active: boolean, forceCheck = false) {
		if (active === this.routeActive && !forceCheck) return
		this.routeActive = active
		if (!active) {
			this.controller?.abort()
			return
		}
		this.recheck(true)
	}

	private recheck(closeSynchronously: boolean) {
		if (!this.mounted || !this.routeActive) return
		// Close private state while checking a newly entered protected route.
		if (
			closeSynchronously &&
			(this.state.phase === 'normal' || this.state.phase === 'checking')
		)
			this.publish({ phase: 'checking', status: null })
		void this.check()
	}

	async check() {
		if (!this.mounted || !this.routeActive) return
		this.controller?.abort()
		const controller = new AbortController()
		this.controller = controller
		try {
			const value = await checkedJSON(
				await this.request('/api/public/mypage-status', {
					method: 'GET',
					signal: AbortSignal.any([
						controller.signal,
						AbortSignal.timeout(8000),
					]),
					credentials: 'same-origin',
					cache: 'no-store',
				}),
			)
			if (controller.signal.aborted || this.controller !== controller) return
			const status = parseMaintenanceStatus(value)
			this.publish({
				phase: status.state === 'ACTIVE' ? 'active' : 'normal',
				status,
			})
		} catch {
			if (controller.signal.aborted || this.controller !== controller) return
			this.publish({ phase: 'unknown', status: null })
		} finally {
			if (this.controller === controller) this.controller = null
		}
	}

	mount() {
		this.mounted = true
		if (this.routeActive) this.recheck(true)
		return () => {
			this.mounted = false
			this.controller?.abort()
			this.listeners.clear()
		}
	}
}
