import { checkedJSON, objectValue } from './features/mypage/client'

export type PublicMaintenanceStatus = {
	state: 'NORMAL' | 'ACTIVE'
	updatedAt: string
	estimatedEndAt?: string
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

const POLL_MS = 120_000

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
	const expected =
		status.estimatedEndAt === undefined
			? 'messageTemplate,state,updatedAt'
			: 'estimatedEndAt,messageTemplate,state,updatedAt'
	if (
		keys !== expected ||
		(status.state !== 'NORMAL' && status.state !== 'ACTIVE') ||
		!timestamp(status.updatedAt) ||
		status.messageTemplate !== 'MYPAGE_MAINTENANCE_V1' ||
		(status.estimatedEndAt !== undefined &&
			!timestamp(status.estimatedEndAt)) ||
		(status.state === 'NORMAL' && status.estimatedEndAt !== undefined)
	)
		throw new Error('invalid maintenance status')
	return status as PublicMaintenanceStatus
}

export class MaintenanceMonitor {
	private state: MaintenanceSnapshot = { phase: 'checking', status: null }
	private listeners = new Set<() => void>()
	private controller: AbortController | null = null
	private timer: ReturnType<typeof setTimeout> | null = null
	private mounted = false
	private routeActive = false
	private visible = true
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
		const previous = this.state
		if (
			state.phase === 'normal' &&
			previous.phase === 'normal' &&
			previous.status.updatedAt !== state.status.updatedAt
		)
			// A changed NORMAL revision can mean this tab missed a complete
			// NORMAL -> ACTIVE -> NORMAL interval. Invalidate private state before
			// publishing the new authoritative snapshot without flashing CHECKING on
			// every unchanged background poll.
			this.closePrivateGate('checking')
		this.state = state
		if (state.phase !== 'normal') this.closePrivateGate(state.phase)
		for (const listener of this.listeners) listener()
	}

	private stopTimer() {
		if (this.timer !== null) clearTimeout(this.timer)
		this.timer = null
	}

	private schedule() {
		this.stopTimer()
		if (!this.mounted || !this.routeActive || !this.visible) return
		this.timer = setTimeout(() => this.recheck(false), POLL_MS)
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
		this.schedule()
	}

	setRouteActive(active: boolean, forceCheck = false) {
		if (active === this.routeActive && !forceCheck) return
		this.routeActive = active
		if (!active) {
			this.controller?.abort()
			this.stopTimer()
			return
		}
		this.recheck(true)
	}

	private recheck(closeSynchronously: boolean) {
		if (!this.mounted || !this.routeActive || !this.visible) return
		// Route and bfcache restoration are synchronous safety boundaries. A
		// routine visible-tab poll keeps a confirmed NORMAL snapshot usable until
		// the new response proves ACTIVE, UNKNOWN, or a changed NORMAL revision.
		if (
			closeSynchronously &&
			(this.state.phase === 'normal' || this.state.phase === 'checking')
		)
			this.publish({ phase: 'checking', status: null })
		void this.check()
	}

	async check() {
		if (!this.mounted || !this.routeActive || !this.visible) return
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
			this.schedule()
		}
	}

	mount() {
		this.mounted = true
		this.visible = document.visibilityState === 'visible'
		const visibility = () => {
			const visible = document.visibilityState === 'visible'
			if (visible === this.visible) return
			this.visible = visible
			// A hidden tab may have missed NORMAL -> ACTIVE. Close private state
			// synchronously before the foreground read proves the current state.
			if (visible && this.routeActive) this.recheck(true)
			else {
				this.controller?.abort()
				this.stopTimer()
			}
		}
		const show = (event: PageTransitionEvent) => {
			if (!event.persisted) return
			this.visible = document.visibilityState === 'visible'
			if (this.visible && this.routeActive) this.recheck(true)
		}
		document.addEventListener('visibilitychange', visibility)
		window.addEventListener('pageshow', show)
		if (this.routeActive && this.visible) this.recheck(true)
		return () => {
			this.mounted = false
			this.controller?.abort()
			this.stopTimer()
			document.removeEventListener('visibilitychange', visibility)
			window.removeEventListener('pageshow', show)
			this.listeners.clear()
		}
	}
}
