type Consent = 'unset' | 'granted' | 'denied'
// v1の選択はGA4未接続時に集めたため、後から有効にしない。
export const consentKey = 'oss.analytics-consent.v2'
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
			/* ストレージが使えなければ無効のままにする。 */
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
				/* 現在のページでの選択は引き続き適用する。 */
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
			/* 無効のままにする。 */
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

// 任意のタイトル・referrer・パラメーター・ユーザーIDは受け取らない。
// クエリとフラグメントは、固定された公開キャンペーンの完全な組を除いて捨てる。
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
				/* 依存先の詳細は出さない。 */
			}
		}
	}
	private send(event: SafeEvent) {
		if (!this.active || this.consent.getSnapshot().value !== 'granted') return
		try {
			this.port?.send(event)
		} catch {
			/* アクセス解析の失敗をログインやデータ取得に波及させない。 */
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

export function createPrivacy(
	existingConsent?: ConsentStore,
	port: AnalyticsPort | null = null,
) {
	let storage: PreferenceStorage | null = null
	try {
		storage = window.localStorage
	} catch {
		/* 利用不能時も同意UIはメモリ内で動く。 */
	}
	const consent = existingConsent ?? new ConsentStore(storage)
	return { consent, analytics: new AnalyticsGate(consent, port) }
}
