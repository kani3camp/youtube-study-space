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
	private authGeneration = 0
	private confirmationGeneration = 0
	private observedUID: string | null | undefined
	private reconciledGeneration = -1
	private restoringController: AbortController | null = null

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

	private observeIdentity(uid: string | null) {
		if (uid !== this.observedUID) {
			this.observedUID = uid
			this.authGeneration++
		}
	}

	private async completeSession(
		uid: string,
		signal: AbortSignal,
		ownsSession: () => boolean,
	) {
		for (let attempt = 0; attempt < 3; attempt++) {
			if (signal.aborted || !ownsSession())
				throw new RequestError(401, 'AUTH_REQUIRED')
			try {
				const response = await this.call(
					'/api/auth/session/complete',
					'POST',
					undefined,
					signal,
					uid,
				)
				if (response !== null)
					throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
				if (signal.aborted || !ownsSession())
					throw new RequestError(401, 'AUTH_REQUIRED')
				return
			} catch (error) {
				if (
					!(error instanceof RequestError) ||
					error.status < 500 ||
					attempt === 2 ||
					signal.aborted ||
					!ownsSession()
				)
					throw error
				await new Promise<void>((resolve) =>
					setTimeout(resolve, 500 * (attempt + 1)),
				)
			}
		}
	}

	private restoreIdentity(uid: string | null) {
		this.restoringController?.abort()
		this.restoringController = null
		if (!this.active || this.completingSession) return
		if (!uid) {
			this.memory.setIdentity(null)
			return
		}
		if (this.reconciledGeneration === this.authGeneration) {
			this.memory.setIdentity(uid)
			return
		}
		this.memory.suspend()
		const controller = new AbortController()
		this.restoringController = controller
		const identityGeneration = this.authGeneration
		const operation = this.confirmationGeneration
		const ownsSession = () =>
			this.restoringController === controller &&
			identityGeneration === this.authGeneration &&
			operation === this.confirmationGeneration &&
			this.session?.currentUID() === uid
		void this.completeSession(uid, controller.signal, ownsSession)
			.then(() => {
				if (!this.active || controller.signal.aborted || !ownsSession()) return
				this.reconciledGeneration = identityGeneration
				this.memory.setIdentity(uid)
			})
			.catch(async () => {
				if (!controller.signal.aborted && ownsSession())
					await this.memory.logout()
			})
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
			// Track identity transitions even while display/fetch is suspended.
			this.observeIdentity(uid)
			if (active && !this.completingSession) this.restoreIdentity(uid)
		})
		const hide = () => {
			active = false
			this.active = false
			this.restoringController?.abort()
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
						this.restoreIdentity(uid)
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
			this.restoringController?.abort()
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
		const budget = AbortSignal.any([signal, AbortSignal.timeout(8000)])
		let onAbort: () => void = () => {}
		const aborted = new Promise<never>((_, reject) => {
			onAbort = () => reject(new RequestError(503, 'TEMPORARY_UNAVAILABLE'))
			budget.addEventListener('abort', onAbort, { once: true })
			if (budget.aborted) onAbort()
		})
		const work = async () => {
			const headers: Record<string, string> = {
				'X-Firebase-AppCheck': await session.appCheck(false),
			}
			if (uid)
				headers.Authorization = `Bearer ${await session.idToken(uid, false)}`
			if (body !== undefined) headers['Content-Type'] = 'application/json'
			if (budget.aborted || (uid && session.currentUID() !== uid))
				throw new RequestError(401, 'AUTH_REQUIRED')
			return checkedJSON(
				await this.request(path, {
					method,
					headers,
					body: body === undefined ? undefined : JSON.stringify(body),
					signal: budget,
					credentials: 'same-origin',
					cache: 'no-store',
				}),
			)
		}
		try {
			return await Promise.race([work(), aborted])
		} catch (error) {
			if (error instanceof RequestError) throw error
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		} finally {
			budget.removeEventListener('abort', onAbort)
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
		const operation = ++this.confirmationGeneration
		this.restoringController?.abort()
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
		if (signal.aborted || operation !== this.confirmationGeneration)
			throw new RequestError(400, 'INVALID_REQUEST')
		let uid: string
		this.completingSession = true
		this.memory.suspend()
		try {
			uid = await this.session.signIn(result.customToken)
		} catch {
			if (operation === this.confirmationGeneration) {
				this.completingSession = false
				this.restoreIdentity(this.session.currentUID())
			}
			throw new RequestError(401, 'AUTH_REQUIRED')
		}
		this.observeIdentity(uid)
		const identityGeneration = this.authGeneration
		const ownsSession = () =>
			operation === this.confirmationGeneration &&
			identityGeneration === this.authGeneration &&
			this.session?.currentUID() === uid
		try {
			await this.completeSession(uid, signal, ownsSession)
			this.completingSession = false
			this.reconciledGeneration = identityGeneration
			if (this.active) this.memory.setIdentity(uid)
		} catch (error) {
			if (operation === this.confirmationGeneration)
				this.completingSession = false
			// UID equality alone cannot identify a logout/relogin to the same uid.
			if (ownsSession()) await this.memory.logout()
			else if (operation === this.confirmationGeneration)
				this.restoreIdentity(this.session.currentUID())
			if (error instanceof RequestError) throw error
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		}
	}
}
