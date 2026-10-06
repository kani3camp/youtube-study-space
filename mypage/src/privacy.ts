export type Consent = 'unset' | 'granted' | 'denied'
export const consentKey = 'oss.analytics-consent.v1'
export type PreferenceStorage = Pick<
	Storage,
	'getItem' | 'setItem' | 'removeItem'
>

export class ConsentStore {
	private state: { value: Consent; saved: boolean } = {
		value: 'unset',
		saved: true,
	}
	private listeners = new Set<() => void>()
	private storage: PreferenceStorage | null
	constructor(storage: PreferenceStorage | null) {
		this.storage = storage
		try {
			const value = storage?.getItem(consentKey)
			if (value === 'granted' || value === 'denied')
				this.state = { value, saved: true }
		} catch {
			/* Storage unavailable: stay off. */
		}
	}
	getSnapshot = () => this.state
	subscribe = (listener: () => void) => {
		this.listeners.add(listener)
		return () => {
			this.listeners.delete(listener)
		}
	}
	set(value: Exclude<Consent, 'unset'>) {
		let saved = false
		try {
			this.storage?.setItem(consentKey, value)
			saved = this.storage !== null
		} catch {
			try {
				this.storage?.removeItem(consentKey)
			} catch {
				/* Current-page choice still applies. */
			}
		}
		this.state = { value, saved }
		for (const listener of this.listeners) listener()
	}
	reload() {
		let value: Consent = 'unset'
		try {
			const stored = this.storage?.getItem(consentKey)
			if (stored === 'granted' || stored === 'denied') value = stored
		} catch {
			/* Stay off. */
		}
		if (this.state.value === value) return
		this.state = { value, saved: true }
		for (const listener of this.listeners) listener()
	}
}

const routes: Record<string, string> = {
	'/': 'オンライン作業部屋',
	'/login': 'ログイン',
	'/login/channel-confirm': 'チャンネル確認',
	'/mypage': 'マイページ',
	'/mypage/account': 'アカウント管理',
	'/privacy': 'プライバシー',
	'/terms': '利用規約',
	'/contact': 'お問い合わせ',
}
const sourcePairs = new Set([
	'channel_profile/profile_link',
	'live_description/description_link',
	'live_chat/pinned_message',
	'live_chat/bot_command',
	'social/community_post',
])
export type SafePage = { page_path: string; page_title: string }
export type AnalyticsEvent =
	| 'login_started'
	| 'channel_confirmation_viewed'
	| 'login'
	| 'mypage_viewed'
export type SafeEvent =
	| { name: 'page_view'; page: SafePage }
	| { name: AnalyticsEvent }
export type AnalyticsPort = {
	start: () => void
	stop: () => void
	send: (event: SafeEvent) => void
}

// No arbitrary title/referrer/parameter/user-ID enters this boundary. Query and
// fragment are dropped except complete, fixed public campaign tuples.
export function safePage(href: string): SafePage | null {
	try {
		const url = new URL(href, 'https://route.invalid')
		if (
			url.origin !== 'https://route.invalid' ||
			!Object.hasOwn(routes, url.pathname)
		)
			return null
		const query = url.searchParams
		const medium = query.get('utm_medium')
		const content = query.get('utm_content')
		const fixedCampaign =
			query.get('utm_source') === 'youtube' &&
			query.get('utm_campaign') === 'mypage' &&
			sourcePairs.has(`${medium}/${content}`) &&
			['utm_source', 'utm_medium', 'utm_campaign', 'utm_content'].every(
				(key) => query.getAll(key).length === 1,
			)
		const safe = fixedCampaign
			? `?utm_source=youtube&utm_medium=${medium}&utm_campaign=mypage&utm_content=${content}`
			: ''
		return {
			page_path: url.pathname + safe,
			page_title: routes[url.pathname] ?? '',
		}
	} catch {
		return null
	}
}

export class AnalyticsGate {
	readonly available: boolean
	private active = false
	private consent: ConsentStore
	private port: AnalyticsPort | null
	constructor(consent: ConsentStore, port: AnalyticsPort | null = null) {
		this.consent = consent
		this.port = port
		this.available = port !== null
	}
	mount() {
		const sync = () => {
			const allow = this.consent.getSnapshot().value === 'granted'
			if (allow === this.active) return
			this.active = allow
			try {
				if (allow) this.port?.start()
				else this.port?.stop()
			} catch {
				this.active = false
			}
		}
		sync()
		const unsubscribe = this.consent.subscribe(sync)
		return () => {
			unsubscribe()
			this.active = false
			try {
				this.port?.stop()
			} catch {
				/* No dependency details. */
			}
		}
	}
	private send(event: SafeEvent) {
		if (!this.active || this.consent.getSnapshot().value !== 'granted') return
		try {
			this.port?.send(event)
		} catch {
			/* Never couple analytics failure to login/data. */
		}
	}
	page(href: string) {
		const page = safePage(href)
		if (page) this.send({ name: 'page_view', page })
	}
	event(name: AnalyticsEvent) {
		if (
			![
				'login_started',
				'channel_confirmation_viewed',
				'login',
				'mypage_viewed',
			].includes(name)
		)
			return
		this.send({ name })
	}
}

export function createPrivacy() {
	let storage: PreferenceStorage | null = null
	try {
		storage = window.localStorage
	} catch {
		/* Consent UI works in memory when unavailable. */
	}
	const consent = new ConsentStore(storage)
	// No GA4 script/ping/identifier exists until an approved transport is wired.
	return { consent, analytics: new AnalyticsGate(consent) }
}
