import { checkedJSON, objectValue } from './features/mypage/client'

export type PublicMaintenanceStatus = {
	maintenanceEnabled: boolean
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

export function parseMaintenanceStatus(
	value: unknown,
): PublicMaintenanceStatus {
	const status = objectValue(value)
	if (!status) throw new Error('invalid maintenance status')
	if (
		Object.keys(status).length !== 1 ||
		typeof status.maintenanceEnabled !== 'boolean'
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
				status: { maintenanceEnabled: true },
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
				phase: status.maintenanceEnabled ? 'active' : 'normal',
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
