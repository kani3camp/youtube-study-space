import { GA4Sender, type GtagCommand, type GtagRuntime } from './ga4'
import {
	type AnalyticsPort,
	type ConsentStore,
	createPrivacy,
	type SafeEvent,
} from './privacy'

export type PreparedTag = {
	runtime: GtagRuntime
	ready: Promise<void>
	cancel: () => void
}
export type TagLoader = (prepare: (runtime: GtagRuntime) => void) => PreparedTag

// Loading never queues product events. Startup commands are prepared before the
// vendor can execute. A superseded load cannot stop a newer consent generation.
export class ConsentTag implements AnalyticsPort {
	private generation = 0
	private current: {
		tag: PreparedTag
		sender: GA4Sender
		ready: boolean
	} | null = null
	private readonly consent: ConsentStore
	private readonly id: string
	private readonly origin: string
	private readonly load: TagLoader
	constructor(
		consent: ConsentStore,
		id: string,
		origin: string,
		load: TagLoader,
	) {
		this.consent = consent
		this.id = id
		this.origin = origin
		this.load = load
	}
	start() {
		this.stop()
		if (this.consent.getSnapshot().value !== 'granted') return
		const generation = this.generation
		let sender: GA4Sender | null = null
		try {
			const tag = this.load((runtime) => {
				if (
					generation !== this.generation ||
					this.consent.getSnapshot().value !== 'granted'
				)
					return
				sender = new GA4Sender(this.id, this.origin, runtime)
				sender.start()
			})
			if (
				!sender ||
				generation !== this.generation ||
				this.consent.getSnapshot().value !== 'granted'
			) {
				tag.cancel()
				return
			}
			const current = { tag, sender, ready: false }
			this.current = current
			void tag.ready.then(
				() => {
					if (
						this.current === current &&
						generation === this.generation &&
						this.consent.getSnapshot().value === 'granted'
					)
						current.ready = true
				},
				() => {
					if (this.current === current) this.stop()
				},
			)
		} catch {
			this.stop()
		}
	}
	stop() {
		this.generation++
		const current = this.current
		this.current = null
		if (!current) return
		try {
			current.sender.stop()
		} finally {
			current.tag.cancel()
		}
	}
	send(event: SafeEvent) {
		if (!this.current?.ready || this.consent.getSnapshot().value !== 'granted')
			return
		this.current.sender.send(event)
	}
}

type Layer = {
	push: (...values: IArguments[]) => number
	splice: (start: number) => unknown
}
export type TagWindow = { ossAnalyticsLayer?: Layer } & Record<string, unknown>

// The caller must complete Console and network-inventory review before wiring
// this browser boundary. Removing a script does not undo vendor code already run.
export function browserTagLoader(
	id: string,
	target: TagWindow,
	doc: Document,
): TagLoader {
	if (!/^G-[A-Z0-9]{6,20}$/.test(id))
		throw new Error('Analytics configuration unavailable')
	if (target.ossAnalyticsLayer || target.dataLayer || target.gtag)
		throw new Error('Analytics configuration unavailable')
	const layer: IArguments[] = []
	let initialized = false
	let loaded = false
	let owner = 0
	function queue(..._args: unknown[]) {
		// biome-ignore lint/complexity/noArguments: gtag.js requires the documented IArguments command shape.
		layer.push(arguments)
	}
	return (prepare) => {
		const token = ++owner
		let cancelled = false
		if (!initialized) {
			target.ossAnalyticsLayer = layer
			initialized = true
		}
		const runtime: GtagRuntime = {
			command: function (..._args: GtagCommand) {
				if (!cancelled && owner === token) {
					if (_args[0] === 'config' && !loaded) queue('js', new Date())
					// biome-ignore lint/complexity/noArguments: gtag.js requires the documented IArguments command shape.
					layer.push(arguments)
				}
			},
			disable: (measurementID, disabled) => {
				if (owner === token) target[`ga-disable-${measurementID}`] = disabled
			},
		}
		const script = doc.createElement('script')
		script.async = true
		script.referrerPolicy = 'no-referrer'
		script.src = `https://www.googletagmanager.com/gtag/js?id=${id}&l=ossAnalyticsLayer`
		let rejectReady: (reason: Error) => void = () => {}
		let timer: ReturnType<typeof setTimeout> | undefined
		const cancel = () => {
			if (cancelled) return
			cancelled = true
			clearTimeout(timer)
			script.onload = null
			script.onerror = null
			script.remove()
			if (owner === token) {
				target[`ga-disable-${id}`] = true
				layer.splice(0)
			}
			rejectReady(new Error('Analytics unavailable'))
		}
		const ready = new Promise<void>((resolve, reject) => {
			rejectReady = reject
			script.onload = () => {
				if (cancelled || owner !== token) return
				clearTimeout(timer)
				loaded = true
				resolve()
			}
			script.onerror = () => {
				cancel()
			}
			try {
				prepare(runtime)
				if (loaded) resolve()
				else {
					timer = setTimeout(cancel, 5000)
					doc.head.append(script)
				}
			} catch {
				cancel()
			}
		})
		return { runtime, ready, cancel }
	}
}

// Public build inputs only. Absent explicit approval leaves the transport null.
export function configuredPrivacy(
	env: Record<string, unknown>,
	target: TagWindow,
	doc: Document,
) {
	const services = createPrivacy()
	if (env.VITE_MYPAGE_ANALYTICS_READY !== 'true') return services
	const id = env.VITE_MYPAGE_GA4_ID
	const origin = env.VITE_MYPAGE_PUBLIC_ORIGIN
	if (
		typeof id !== 'string' ||
		typeof origin !== 'string' ||
		doc.location.origin !== origin
	)
		return services
	try {
		// Validate before creating a loader or browser global.
		new GA4Sender(id, origin, { command: () => {}, disable: () => {} })
		const tag = new ConsentTag(
			services.consent,
			id,
			origin,
			browserTagLoader(id, target, doc),
		)
		return createPrivacy(services.consent, tag)
	} catch {
		return services
	}
}
