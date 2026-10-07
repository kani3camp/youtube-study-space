import type {
	Account,
	Current,
	Metric,
	MyPage,
	ReasonCode,
	Section,
} from './contract'

export type Retained<T> = {
	data: T | null
	asOf: string | null
	stale: boolean
	reason: ReasonCode | null
}
export type ViewData = {
	current: Retained<Current>
	account: Retained<Account>
	summary: Record<'today' | 'week' | 'lifetime', Retained<number>>
	recent: (Retained<number> & { date: string })[]
	generatedAt: string
}
export type MemoryState = {
	phase:
		| 'bootstrapping'
		| 'anonymous'
		| 'authenticated'
		| 'signing-out'
		| 'restricted'
	data: ViewData | null
	busy: boolean
	error: string | null
	receivedAt: number | null
}

export class RequestError extends Error {
	readonly status: number
	readonly code: string
	readonly retryAfter: number
	constructor(status: number, code: string, retryAfter = 0) {
		super('MyPage request failed')
		this.status = status
		this.code = code
		this.retryAfter = retryAfter
	}
}

export type RestrictionCode =
	| 'SERVICE_ACCESS_RESTRICTED'
	| 'DATA_DELETION_IN_PROGRESS'

export function restrictionFrom(error: unknown): RestrictionCode | null {
	return error instanceof RequestError &&
		error.status === 403 &&
		(error.code === 'SERVICE_ACCESS_RESTRICTED' ||
			error.code === 'DATA_DELETION_IN_PROGRESS')
		? error.code
		: null
}

export type Loader = (
	uid: string,
	signal: AbortSignal,
	forceToken: boolean,
	forceAppCheck: boolean,
) => Promise<MyPage>

function retain<T>(
	previous: Retained<T> | undefined,
	section: Section<T>,
	asOf: string,
): Retained<T> {
	if (section.data !== null)
		return {
			data: section.data,
			asOf,
			stale: section.availability !== 'available',
			reason: section.reasonCode,
		}
	return {
		data: previous?.data ?? null,
		asOf: previous?.asOf ?? null,
		stale: true,
		reason: section.reasonCode,
	}
}

function retainMetric(
	previous: Retained<number> | undefined,
	metric: Metric | undefined,
	asOf: string,
): Retained<number> {
	if (metric?.availability === 'available')
		return { data: metric.workSec, asOf, stale: false, reason: null }
	return {
		data: previous?.data ?? null,
		asOf: previous?.asOf ?? null,
		stale: true,
		reason: metric?.reasonCode ?? 'SOURCE_UNAVAILABLE',
	}
}

export function mergeResponse(
	previous: ViewData | null,
	next: MyPage,
): ViewData {
	return {
		current: retain(previous?.current, next.current, next.generatedAt),
		account: retain(
			next.account.reasonCode === 'METADATA_TOO_OLD'
				? undefined
				: previous?.account,
			next.account,
			next.generatedAt,
		),
		summary: {
			today: retainMetric(
				previous?.summary.today,
				next.summary.data?.today,
				next.generatedAt,
			),
			week: retainMetric(
				previous?.summary.week,
				next.summary.data?.week,
				next.generatedAt,
			),
			lifetime: retainMetric(
				previous?.summary.lifetime,
				next.summary.data?.lifetime,
				next.generatedAt,
			),
		},
		recent: next.recent7Days.data.map((day) => ({
			date: day.date,
			...retainMetric(
				previous?.recent.find((item) => item.date === day.date),
				day,
				next.generatedAt,
			),
		})),
		generatedAt: next.generatedAt,
	}
}

// Private data lives only in this instance. Epoch changes make delayed responses
// harmless even when a fetch implementation ignores AbortSignal.
export class MyPageMemory {
	private state: MemoryState = {
		phase: 'bootstrapping',
		data: null,
		busy: false,
		error: null,
		receivedAt: null,
	}
	private uid: string | null = null
	private epoch = 0
	private listeners = new Set<() => void>()
	private controller: AbortController | null = null
	private timer: ReturnType<typeof setTimeout> | null = null
	private visible = true
	private failures = 0
	private ignoreAuthUntilSignedOut = false
	private lastManual = Number.NEGATIVE_INFINITY
	private retryAt = 0

	private load: Loader
	private signOut: () => Promise<void>
	private now: () => number
	constructor(load: Loader, signOut: () => Promise<void>, now = Date.now) {
		this.load = load
		this.signOut = signOut
		this.now = now
	}

	getSnapshot = () => this.state
	get generation() {
		return this.epoch
	}
	subscribe = (listener: () => void) => {
		this.listeners.add(listener)
		return () => this.listeners.delete(listener)
	}

	private publish(state: MemoryState) {
		this.state = state
		for (const listener of this.listeners) listener()
	}
	private stop() {
		if (this.timer !== null) clearTimeout(this.timer)
		this.timer = null
		this.controller?.abort()
		this.controller = null
	}
	private clear(phase: MemoryState['phase'], error: string | null = null) {
		this.epoch++
		this.stop()
		this.failures = 0
		this.publish({
			phase,
			data: null,
			busy: false,
			error,
			receivedAt: null,
		})
	}

	setIdentity(uid: string | null, freshLogin = false) {
		if (this.ignoreAuthUntilSignedOut && uid !== null) return
		if (uid === null) this.ignoreAuthUntilSignedOut = false
		if (
			uid === this.uid &&
			this.state.phase !== 'bootstrapping' &&
			!(freshLogin && this.state.phase === 'restricted')
		)
			return
		this.uid = uid
		this.clear(uid === null ? 'anonymous' : 'authenticated')
		if (uid !== null) void this.refresh()
	}

	isRestricted(uid: string | null) {
		return this.state.phase === 'restricted' && uid === this.uid
	}

	// A definitive denial is terminal for this identity until a fresh login.
	// Clearing increments the epoch before publishing, rejecting late responses.
	restrict(code: RestrictionCode, uid = this.uid) {
		this.uid = uid
		this.clear('restricted', code)
	}

	setVisible(visible: boolean) {
		if (visible === this.visible) return
		this.visible = visible
		if (!visible) {
			if (this.timer !== null) clearTimeout(this.timer)
			this.timer = null
		} else void this.refresh()
	}

	// pagehide clears pixels and cached private values before a bfcache snapshot.
	suspend(freshLogin = false) {
		if (this.state.phase === 'restricted' && !freshLogin) {
			this.clear('restricted', this.state.error)
			return
		}
		this.uid = null
		this.clear('bootstrapping')
	}

	async logout() {
		this.ignoreAuthUntilSignedOut = true
		this.uid = null
		this.clear('signing-out')
		const epoch = this.epoch
		try {
			await this.signOut()
			if (epoch !== this.epoch) return
			this.ignoreAuthUntilSignedOut = false
			this.publish({ ...this.state, phase: 'anonymous' })
		} catch {
			if (epoch !== this.epoch) return
			this.publish({
				...this.state,
				phase: 'anonymous',
				error: 'LOGOUT_FAILED',
			})
		}
	}

	private schedule(delay: number) {
		if (!this.visible || this.uid === null || this.state.phase === 'restricted')
			return
		if (this.timer !== null) clearTimeout(this.timer)
		this.timer = setTimeout(
			() => {
				this.timer = null
				void this.refresh()
			},
			Math.max(delay, this.retryAt - this.now()),
		)
	}

	async refresh(manual = false) {
		if (
			!this.visible ||
			this.uid === null ||
			this.state.busy ||
			this.state.phase === 'restricted'
		)
			return
		const remaining = this.retryAt - this.now()
		if (remaining > 0) {
			this.schedule(remaining)
			return
		}
		if (manual && this.now() - this.lastManual < 2000) return
		if (manual) this.lastManual = this.now()
		if (this.timer !== null) clearTimeout(this.timer)
		this.timer = null
		const uid = this.uid
		const epoch = this.epoch
		const controller = new AbortController()
		this.controller = controller
		this.publish({ ...this.state, busy: true, error: null })
		let delay = 60_000
		try {
			let forceToken = false
			let forceAppCheck = false
			let response: MyPage
			for (;;) {
				try {
					response = await this.load(
						uid,
						controller.signal,
						forceToken,
						forceAppCheck,
					)
					break
				} catch (error) {
					if (epoch !== this.epoch) return
					if (
						error instanceof RequestError &&
						error.status === 401 &&
						!forceToken
					) {
						// Clear values before forcing a token refresh.
						this.publish({ ...this.state, data: null, receivedAt: null })
						forceToken = true
						continue
					}
					if (
						error instanceof RequestError &&
						error.code === 'APP_CHECK_REQUIRED' &&
						!forceAppCheck
					) {
						forceAppCheck = true
						continue
					}
					throw error
				}
			}
			if (epoch !== this.epoch || uid !== this.uid) return
			this.failures = 0
			this.publish({
				...this.state,
				data: mergeResponse(this.state.data, response),
				receivedAt: this.now(),
			})
		} catch (error) {
			if (epoch !== this.epoch) return
			const restriction = restrictionFrom(error)
			if (restriction) {
				this.restrict(restriction, uid)
				return
			}
			if (
				error instanceof RequestError &&
				(error.status === 401 ||
					error.code === 'PRIVACY_RECONSENT_REQUIRED' ||
					error.code === 'WEB_ACCOUNT_REQUIRED')
			) {
				const code = error.code
				await this.logout()
				if (
					this.state.phase === 'anonymous' &&
					this.state.error !== 'LOGOUT_FAILED'
				)
					this.publish({ ...this.state, error: code })
				return
			}
			this.failures++
			delay = Math.min(300_000, 60_000 * 2 ** Math.min(this.failures - 1, 3))
			if (
				error instanceof RequestError &&
				error.status === 429 &&
				Number.isFinite(error.retryAfter)
			) {
				const cooldown = Math.min(3600, Math.max(0, error.retryAfter)) * 1000
				this.retryAt = Math.max(this.retryAt, this.now() + cooldown)
				delay = Math.max(delay, cooldown)
			}
			this.publish({
				...this.state,
				error:
					error instanceof RequestError ? error.code : 'TEMPORARY_UNAVAILABLE',
			})
		} finally {
			if (epoch === this.epoch) {
				this.controller = null
				this.publish({ ...this.state, busy: false })
				this.schedule(delay)
			}
		}
	}

	dispose() {
		this.uid = null
		this.clear('anonymous')
		this.listeners.clear()
	}
}
