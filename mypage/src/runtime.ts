import {
	checkedJSON,
	createLoader,
	type TokenSource,
} from './features/mypage/client'
import {
	MyPageMemory,
	RequestError,
	restrictionFrom,
} from './features/mypage/memory'

export type BrowserSession = TokenSource & {
	subscribe: (listener: (uid: string | null) => void) => () => void
	recheck: () => Promise<string | null>
	signIn: (customToken: string) => Promise<string>
	signOut: () => Promise<void>
}
export type SupportPurpose = 'delete' | 'revoke' | 'disclosure'
export type PrivacyIntakeReceipt = {
	requestRef: string
	supportChallenge?: string
	purpose: SupportPurpose
	status: string
	acceptedAt: string
	deleteBy: string | null
}
export type PrivacyRequestStatus = {
	requestRef: string
	purpose: SupportPurpose
	status: string
	acceptedAt: string
	deleteBy: string | null
	verifiedAt: string
	reply?: string
	replyAt?: string
}
export type ConfirmationResult =
	| { purpose: 'login' }
	| { purpose: 'support'; requestRef: string; supportRequestRef?: string }
export type ChannelConfirmation = (
	| { purpose: 'login' }
	| { purpose: 'support'; supportPurpose: SupportPurpose }
) & {
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
	readonly intakeEnabled: boolean
	private request: typeof fetch
	private completingSession = false
	private active = false
	private authGeneration = 0
	private confirmationGeneration = 0
	private observedUID: string | null | undefined
	private reconciledGeneration = -1
	private restoringController: AbortController | null = null
	private supportController: AbortController | null = null
	private lifecycleGeneration = 0

	private invalidateSupport() {
		this.supportController?.abort()
		this.supportController = null
	}

	constructor(
		session: BrowserSession | null,
		policy: { privacy: string; terms: string },
		request: typeof fetch = fetch,
		unavailable: string | null = null,
		intakeEnabled = false,
	) {
		this.session = session
		this.policy = policy
		this.request = request
		this.unavailable = unavailable
		this.intakeEnabled = intakeEnabled
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
			this.invalidateSupport()
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
		if (this.memory.isRestricted(uid)) return
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
			.catch(async (error) => {
				if (!controller.signal.aborted && ownsSession()) {
					const restriction = restrictionFrom(error)
					if (restriction) this.memory.restrict(restriction, uid)
					else await this.memory.logout()
				}
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
			this.lifecycleGeneration++
			this.invalidateSupport()
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
			this.lifecycleGeneration++
			this.invalidateSupport()
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
			const result = await checkedJSON(
				await this.request(path, {
					method,
					headers,
					body: body === undefined ? undefined : JSON.stringify(body),
					signal: budget,
					credentials: 'same-origin',
					cache: 'no-store',
				}),
			)
			if (budget.aborted || (uid && session.currentUID() !== uid))
				throw new RequestError(401, 'AUTH_REQUIRED')
			return result
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

	async submitPrivacyRequest(
		purpose: SupportPurpose,
		body: string,
		submissionKey: string,
		signal: AbortSignal,
	): Promise<PrivacyIntakeReceipt> {
		const uid = this.session?.currentUID()
		if (!this.intakeEnabled || !uid)
			throw new RequestError(401, 'AUTH_REQUIRED')
		const value = await this.call(
			'/api/privacy/requests',
			'POST',
			{ purpose, body, submissionKey },
			signal,
			uid,
		)
		if (
			!value ||
			typeof value !== 'object' ||
			!('requestRef' in value) ||
			typeof value.requestRef !== 'string' ||
			!/^[a-f0-9]{64}$/.test(value.requestRef) ||
			!('acceptedAt' in value) ||
			typeof value.acceptedAt !== 'string'
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		return value as PrivacyIntakeReceipt
	}

	async privacyRequestStatus(
		requestRef: string,
		signal: AbortSignal,
	): Promise<PrivacyRequestStatus> {
		const uid = this.session?.currentUID()
		if (!this.intakeEnabled || !uid)
			throw new RequestError(401, 'AUTH_REQUIRED')
		if (!/^[a-f0-9]{64}$/.test(requestRef))
			throw new RequestError(400, 'SUPPORT_CHALLENGE_INVALID')
		const value = await this.call(
			'/api/privacy/requests/status',
			'POST',
			{ requestRef },
			signal,
			uid,
		)
		if (
			!value ||
			typeof value !== 'object' ||
			!('requestRef' in value) ||
			value.requestRef !== requestRef ||
			!('status' in value) ||
			value.status !== 'verified'
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		return value as PrivacyRequestStatus
	}

	async start(signal: AbortSignal, supportChallenge?: string) {
		if (
			supportChallenge !== undefined &&
			!/^[a-f0-9]{64}$/.test(supportChallenge)
		)
			throw new RequestError(400, 'SUPPORT_CHALLENGE_INVALID')
		const result = await this.call(
			'/api/auth/youtube/start',
			'POST',
			{
				...(supportChallenge === undefined ? {} : { supportChallenge }),
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
		const identityGeneration = this.authGeneration
		const lifecycleGeneration = this.lifecycleGeneration
		const memoryGeneration = this.memory.generation
		let result: unknown
		try {
			result = await this.call(
				'/api/auth/youtube/channel',
				'GET',
				undefined,
				signal,
			)
		} catch (error) {
			if (
				signal.aborted ||
				identityGeneration !== this.authGeneration ||
				lifecycleGeneration !== this.lifecycleGeneration
			)
				throw new RequestError(401, 'AUTH_REQUIRED')
			const restriction = restrictionFrom(error)
			if (restriction)
				this.memory.restrict(restriction, this.session?.currentUID() ?? null)
			throw error
		}
		if (
			signal.aborted ||
			identityGeneration !== this.authGeneration ||
			lifecycleGeneration !== this.lifecycleGeneration
		)
			throw new RequestError(401, 'AUTH_REQUIRED')
		if (
			!result ||
			typeof result !== 'object' ||
			!('purpose' in result) ||
			!(
				result.purpose === 'login' ||
				(result.purpose === 'support' &&
					'supportPurpose' in result &&
					['delete', 'revoke', 'disclosure'].includes(
						String(result.supportPurpose),
					))
			) ||
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
		if (
			result.purpose === 'login' &&
			this.memory.getSnapshot().phase === 'restricted' &&
			memoryGeneration !== this.memory.generation
		)
			throw new RequestError(
				403,
				this.memory.getSnapshot().error ?? 'SERVICE_ACCESS_RESTRICTED',
			)
		const metadata = {
			displayName: result.displayName,
			handle: result.handle,
			avatarUrl: result.avatarUrl,
			confirmationRef: result.confirmationRef,
		}
		if (result.purpose === 'support' && 'supportPurpose' in result) {
			return {
				...metadata,
				purpose: 'support',
				supportPurpose: result.supportPurpose as SupportPurpose,
			}
		}
		return { ...metadata, purpose: 'login' }
	}

	async confirm(
		confirmationRef: string,
		signal: AbortSignal,
		purpose: 'login' | 'support' = 'login',
	): Promise<ConfirmationResult> {
		this.invalidateSupport()
		const supportController =
			purpose === 'support' ? new AbortController() : null
		this.supportController = supportController
		const identityGenerationAtStart = this.authGeneration
		const lifecycleGeneration = this.lifecycleGeneration
		const memoryGeneration = this.memory.generation
		const uidAtStart = this.session?.currentUID()
		const supportSignal = supportController
			? AbortSignal.any([signal, supportController.signal])
			: signal
		const ownsSupport = () =>
			!supportSignal.aborted &&
			this.supportController === supportController &&
			identityGenerationAtStart === this.authGeneration &&
			lifecycleGeneration === this.lifecycleGeneration &&
			this.session?.currentUID() === uidAtStart
		const operation =
			purpose === 'login'
				? ++this.confirmationGeneration
				: this.confirmationGeneration
		if (purpose === 'login') this.restoringController?.abort()
		if (!/^[a-f0-9]{64}$/.test(confirmationRef))
			throw new RequestError(400, 'INVALID_REQUEST')
		const ownsConfirmation = () =>
			!signal.aborted &&
			operation === this.confirmationGeneration &&
			identityGenerationAtStart === this.authGeneration &&
			lifecycleGeneration === this.lifecycleGeneration &&
			memoryGeneration === this.memory.generation &&
			this.session?.currentUID() === uidAtStart
		let result: unknown
		try {
			result = await this.call(
				'/api/auth/youtube/confirm',
				'POST',
				{ confirmationRef },
				supportSignal,
			)
		} catch (error) {
			if (purpose === 'login' && !ownsConfirmation())
				throw new RequestError(401, 'AUTH_REQUIRED')
			const restriction = restrictionFrom(error)
			if (purpose === 'login' && restriction && ownsConfirmation())
				this.memory.restrict(restriction, uidAtStart ?? null)
			throw error
		}
		if (signal.aborted) throw new RequestError(400, 'INVALID_REQUEST')
		if (purpose === 'support') {
			if (!ownsSupport()) throw new RequestError(400, 'INVALID_REQUEST')
			if (
				!result ||
				typeof result !== 'object' ||
				!('purpose' in result) ||
				result.purpose !== 'support' ||
				!('requestRef' in result) ||
				typeof result.requestRef !== 'string' ||
				!/^[a-f0-9]{64}$/.test(result.requestRef) ||
				('supportRequestRef' in result &&
					(typeof result.supportRequestRef !== 'string' ||
						!/^[a-f0-9]{64}$/.test(result.supportRequestRef) ||
						result.supportRequestRef === result.requestRef)) ||
				'customToken' in result
			)
				throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
			return {
				purpose: 'support',
				requestRef: result.requestRef,
				...('supportRequestRef' in result
					? { supportRequestRef: result.supportRequestRef as string }
					: {}),
			}
		}
		if (
			!result ||
			typeof result !== 'object' ||
			!('purpose' in result) ||
			result.purpose !== 'login' ||
			'requestRef' in result ||
			'supportRequestRef' in result ||
			!('customToken' in result) ||
			typeof result.customToken !== 'string' ||
			result.customToken === '' ||
			!this.session
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		if (!ownsConfirmation()) throw new RequestError(400, 'INVALID_REQUEST')
		let uid: string
		this.completingSession = true
		this.memory.suspend(true)
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
			lifecycleGeneration === this.lifecycleGeneration &&
			this.session?.currentUID() === uid
		try {
			await this.completeSession(uid, signal, ownsSession)
			this.completingSession = false
			this.reconciledGeneration = identityGeneration
			if (this.active) this.memory.setIdentity(uid, true)
			return { purpose: 'login' }
		} catch (error) {
			const ownedAtFailure = ownsSession()
			if (operation === this.confirmationGeneration)
				this.completingSession = false
			// UID equality alone cannot identify a logout/relogin to the same uid.
			if (ownedAtFailure) {
				const restriction = restrictionFrom(error)
				if (restriction) this.memory.restrict(restriction, uid)
				else await this.memory.logout()
			} else if (operation === this.confirmationGeneration)
				this.restoreIdentity(this.session.currentUID())
			if (!ownedAtFailure) throw new RequestError(401, 'AUTH_REQUIRED')
			if (error instanceof RequestError) throw error
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		}
	}
}
