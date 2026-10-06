import {
	checkedJSON,
	createLoader,
	type TokenSource,
} from './features/mypage/client'
import { MyPageMemory, RequestError } from './features/mypage/memory'

export type BrowserSession = TokenSource & {
	subscribe: (listener: (uid: string | null) => void) => () => void
	recheck: () => Promise<string | null>
	signIn: (customToken: string) => Promise<string>
	signOut: () => Promise<void>
}
export type ChannelConfirmation = {
	purpose: 'login'
	displayName: string
	handle: string | null
	avatarUrl: string | null
	confirmationRef: string
}

export class BrowserRuntime {
	readonly memory: MyPageMemory
	readonly session: BrowserSession | null
	readonly policy: { privacy: string; terms: string }
	readonly unavailable: string | null
	private request: typeof fetch
	private completingSession = false
	private active = false

	constructor(
		session: BrowserSession | null,
		policy: { privacy: string; terms: string },
		request: typeof fetch = fetch,
		unavailable: string | null = null,
	) {
		this.session = session
		this.policy = policy
		this.request = request
		this.unavailable = unavailable
		this.memory = new MyPageMemory(
			session
				? createLoader(session, request)
				: async () => {
						throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
					},
			async () => {
				await session?.signOut()
			},
		)
	}

	mount() {
		if (!this.session) {
			this.memory.setIdentity(null)
			return () => this.memory.dispose()
		}
		const session = this.session
		let active = true
		this.active = true
		let generation = 0
		const visible = () =>
			this.memory.setVisible(document.visibilityState === 'visible')
		visible()
		const unsubscribe = session.subscribe((uid) => {
			if (active && !this.completingSession) this.memory.setIdentity(uid)
		})
		const hide = () => {
			active = false
			this.active = false
			generation++
			this.memory.suspend()
		}
		const show = () => {
			this.memory.setVisible(document.visibilityState === 'visible')
			active = true
			this.active = true
			const current = ++generation
			void session
				.recheck()
				.then((uid) => {
					if (
						active &&
						!this.completingSession &&
						current === generation &&
						session.currentUID() === uid
					)
						this.memory.setIdentity(uid)
				})
				.catch(() => {
					if (active && current === generation) this.memory.setIdentity(null)
				})
		}
		visible()
		document.addEventListener('visibilitychange', visible)
		window.addEventListener('pagehide', hide)
		window.addEventListener('pageshow', show)
		return () => {
			active = false
			this.active = false
			generation++
			unsubscribe()
			document.removeEventListener('visibilitychange', visible)
			window.removeEventListener('pagehide', hide)
			window.removeEventListener('pageshow', show)
			this.memory.dispose()
		}
	}

	private async call(
		path: string,
		method: 'GET' | 'POST',
		body: unknown,
		signal: AbortSignal,
		uid?: string,
	) {
		const session = this.session
		if (!session) throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		try {
			const headers: Record<string, string> = {
				'X-Firebase-AppCheck': await session.appCheck(false),
			}
			if (uid)
				headers.Authorization = `Bearer ${await session.idToken(uid, false)}`
			if (body !== undefined) headers['Content-Type'] = 'application/json'
			if (signal.aborted || (uid && session.currentUID() !== uid))
				throw new RequestError(401, 'AUTH_REQUIRED')
			return await checkedJSON(
				await this.request(path, {
					method,
					headers,
					body: body === undefined ? undefined : JSON.stringify(body),
					signal: AbortSignal.any([signal, AbortSignal.timeout(8000)]),
					credentials: 'same-origin',
					cache: 'no-store',
				}),
			)
		} catch (error) {
			if (error instanceof RequestError) throw error
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		}
	}

	async start(signal: AbortSignal) {
		const result = await this.call(
			'/api/auth/youtube/start',
			'POST',
			{
				privacyPolicyVersion: this.policy.privacy,
				privacyAccepted: true,
				termsVersion: this.policy.terms,
				termsAccepted: true,
			},
			signal,
		)
		if (
			!result ||
			typeof result !== 'object' ||
			!('authorizationUrl' in result) ||
			typeof result.authorizationUrl !== 'string'
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		const url = new URL(result.authorizationUrl)
		if (
			url.origin !== 'https://accounts.google.com' ||
			url.pathname !== '/o/oauth2/v2/auth' ||
			url.username ||
			url.password
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		return url.href
	}

	async channel(signal: AbortSignal): Promise<ChannelConfirmation> {
		const result = await this.call(
			'/api/auth/youtube/channel',
			'GET',
			undefined,
			signal,
		)
		if (
			!result ||
			typeof result !== 'object' ||
			!('purpose' in result) ||
			result.purpose !== 'login' ||
			!('confirmationRef' in result) ||
			typeof result.confirmationRef !== 'string' ||
			!/^[a-f0-9]{64}$/.test(result.confirmationRef) ||
			!('displayName' in result) ||
			typeof result.displayName !== 'string' ||
			result.displayName === '' ||
			!('handle' in result) ||
			!(result.handle === null || typeof result.handle === 'string') ||
			!('avatarUrl' in result) ||
			!(
				result.avatarUrl === null ||
				(typeof result.avatarUrl === 'string' &&
					result.avatarUrl.startsWith('https://'))
			)
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		return result as ChannelConfirmation
	}

	async confirm(confirmationRef: string, signal: AbortSignal) {
		if (!/^[a-f0-9]{64}$/.test(confirmationRef))
			throw new RequestError(400, 'INVALID_REQUEST')
		const result = await this.call(
			'/api/auth/youtube/confirm',
			'POST',
			{ confirmationRef },
			signal,
		)
		if (
			!result ||
			typeof result !== 'object' ||
			!('purpose' in result) ||
			result.purpose !== 'login' ||
			!('customToken' in result) ||
			typeof result.customToken !== 'string' ||
			result.customToken === '' ||
			!this.session
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		if (signal.aborted) throw new RequestError(400, 'INVALID_REQUEST')
		let uid: string
		this.completingSession = true
		this.memory.suspend()
		try {
			uid = await this.session.signIn(result.customToken)
		} catch {
			this.completingSession = false
			if (this.active) this.memory.setIdentity(this.session.currentUID())
			throw new RequestError(401, 'AUTH_REQUIRED')
		}
		try {
			if (signal.aborted || this.session.currentUID() !== uid)
				throw new RequestError(401, 'AUTH_REQUIRED')
			for (let attempt = 0; attempt < 3; attempt++) {
				try {
					await this.call(
						'/api/auth/session/complete',
						'POST',
						undefined,
						signal,
						uid,
					)
					if (signal.aborted || this.session.currentUID() !== uid)
						throw new RequestError(401, 'AUTH_REQUIRED')
					this.completingSession = false
					if (this.active) this.memory.setIdentity(uid)
					return
				} catch (error) {
					if (
						!(error instanceof RequestError) ||
						error.status < 500 ||
						attempt === 2 ||
						signal.aborted ||
						this.session.currentUID() !== uid
					)
						throw error
					await new Promise<void>((resolve) =>
						setTimeout(resolve, 500 * (attempt + 1)),
					)
				}
			}
		} catch (error) {
			this.completingSession = false
			// A late completion must never sign out a newer identity.
			if (this.session.currentUID() === uid) await this.memory.logout()
			else if (this.active) this.memory.setIdentity(this.session.currentUID())
			if (error instanceof RequestError) throw error
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		}
	}
}
