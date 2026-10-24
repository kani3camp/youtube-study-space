import {
	type AnalyticsPort,
	type SafeEvent,
	type SafePage,
	safePage,
} from './privacy'

export type GtagCommand =
	| ['consent', 'default', Record<string, string>]
	| ['config', string, Record<string, string | boolean>]
	| ['event', string, Record<string, string | boolean>]

// Synchronous injection only: no script loader, dataLayer queue, browser globals,
// identifier generation or vendor request exists in this module. A future loader
// must recheck consent before initialization and discard late loaded callbacks.
export type GtagRuntime = {
	command: (...args: GtagCommand) => void
	disable: (measurementID: string, disabled: boolean) => void
}

export class GA4Sender implements AnalyticsPort {
	private active = false
	private page: SafePage | null = null
	private measurementID: string
	private publicOrigin: string
	private runtime: GtagRuntime
	constructor(
		measurementID: string,
		publicOrigin: string,
		runtime: GtagRuntime,
	) {
		let origin: URL
		try {
			origin = new URL(publicOrigin)
		} catch {
			throw new Error('Analytics configuration unavailable')
		}
		if (
			!/^G-[A-Z0-9]{6,20}$/.test(measurementID) ||
			origin.protocol !== 'https:' ||
			origin.origin !== publicOrigin ||
			origin.username ||
			origin.password
		) {
			throw new Error('Analytics configuration unavailable')
		}
		this.measurementID = measurementID
		this.publicOrigin = publicOrigin
		this.runtime = runtime
	}
	start() {
		if (this.active) return
		try {
			this.runtime.disable(this.measurementID, false)
			this.runtime.command('consent', 'default', {
				analytics_storage: 'granted',
				ad_storage: 'denied',
				ad_user_data: 'denied',
				ad_personalization: 'denied',
			})
			this.runtime.command('config', this.measurementID, {
				send_page_view: false,
				allow_google_signals: false,
				allow_ad_personalization_signals: false,
				page_location: `${this.publicOrigin}/`,
				page_referrer: '',
				page_title: '',
			})
			this.active = true
		} catch {
			this.stop()
		}
	}
	stop() {
		this.active = false
		this.page = null
		// Do not issue consent-denied/config commands that could trigger a ping.
		// A future runtime must disable its vendor before any pending processing.
		this.runtime.disable(this.measurementID, true)
	}
	send(event: SafeEvent) {
		if (!this.active) return
		if (event.name === 'page_view') {
			// Revalidate at the transport boundary; ignore supplied title and extras.
			const page = safePage(event.page.page_path)
			if (!page) return
			this.page = page
		} else if (
			![
				'login_started',
				'channel_confirmation_viewed',
				'login',
				'mypage_viewed',
			].includes(event.name)
		) {
			return
		}
		this.runtime.command('event', event.name, {
			send_to: this.measurementID,
			page_location: this.publicOrigin + (this.page?.page_path ?? '/'),
			page_title: this.page?.page_title ?? '',
			page_referrer: '',
			allow_google_signals: false,
			allow_ad_personalization_signals: false,
		})
	}
}
