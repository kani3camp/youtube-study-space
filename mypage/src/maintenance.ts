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
	return (
		typeof value === 'string' &&
		/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
			value,
		) &&
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
		this.timer = setTimeout(() => this.recheck(), POLL_MS)
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
		this.recheck()
	}

	private recheck() {
		if (!this.mounted || !this.routeActive || !this.visible) return
		// Close the private-data gate synchronously when the previous status could
		// still permit personal data. ACTIVE and UNKNOWN are already safe states;
		// preserve their stable UI while a background poll is in flight.
		if (this.state.phase === 'normal' || this.state.phase === 'checking')
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
			if (visible && this.routeActive) this.recheck()
			else {
				this.controller?.abort()
				this.stopTimer()
			}
		}
		const show = () => {
			this.visible = document.visibilityState === 'visible'
			if (this.visible && this.routeActive) this.recheck()
		}
		document.addEventListener('visibilitychange', visibility)
		window.addEventListener('pageshow', show)
		if (this.routeActive && this.visible) this.recheck()
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
