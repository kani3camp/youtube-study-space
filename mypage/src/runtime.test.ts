import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../docs/mypage/fixtures/available.json'
import { RequestError } from './features/mypage/memory'
import { BrowserRuntime, type BrowserSession } from './runtime'

function sessionFixture() {
	let uid: string | null = null
	const listeners = new Set<(uid: string | null) => void>()
	const setUID = (next: string | null) => {
		uid = next
		for (const listener of listeners) listener(next)
	}
	const session: BrowserSession = {
		currentUID: () => uid,
		idToken: async (wanted) => {
			if (uid !== wanted) throw new RequestError(401, 'AUTH_REQUIRED')
			return 'synthetic-id'
		},
		appCheck: async () => 'synthetic-app',
		subscribe: (listener) => {
			listeners.add(listener)
			listener(uid)
			return () => {
				listeners.delete(listener)
			}
		},
		recheck: vi.fn(async () => uid),
		signIn: vi.fn(async () => {
			setUID('synthetic')
			return 'synthetic'
		}),
		signOut: vi.fn(async () => setUID(null)),
	}
	return { session, setUID }
}
async function settle() {
	for (let i = 0; i < 12; i++) await Promise.resolve()
}

afterEach(() => {
	vi.useRealTimers()
	vi.unstubAllGlobals()
})

describe('browser authentication runtime', () => {
	it('mints once, signs in once and retries only transient session completion', async () => {
		vi.useFakeTimers()
		const { session } = sessionFixture()
		let complete = 0
		const request = vi.fn<typeof fetch>(async (path) => {
			if (path === '/api/auth/youtube/confirm')
				return Response.json({
					purpose: 'login',
					customToken: 'synthetic-custom',
				})
			if (path === '/api/auth/session/complete') {
				complete++
				return complete < 3
					? Response.json(
							{ error: { code: 'TEMPORARY_UNAVAILABLE' } },
							{ status: 503 },
						)
					: new Response(null, { status: 204 })
			}
			return Response.json(fixture)
		})
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const done = runtime.confirm('a'.repeat(64), new AbortController().signal)
		await vi.runAllTimersAsync()
		await done
		expect(
			request.mock.calls.filter(
				(call) => call[0] === '/api/auth/youtube/confirm',
			),
		).toHaveLength(1)
		expect(session.signIn).toHaveBeenCalledExactlyOnceWith('synthetic-custom')
		expect(complete).toBe(3)
		expect(session.signOut).not.toHaveBeenCalled()
		expect(JSON.stringify(runtime.memory.getSnapshot())).not.toContain(
			'synthetic-custom',
		)
		runtime.memory.dispose()
	})
	it('signs out on missing WebAccount, preserves signout failure and allows retry', async () => {
		const { session } = sessionFixture()
		const signOut = vi
			.fn()
			.mockRejectedValueOnce(new Error('synthetic failure'))
			.mockResolvedValueOnce(undefined)
		session.signOut = signOut
		const request = vi.fn<typeof fetch>(async (path) =>
			path === '/api/auth/youtube/confirm'
				? Response.json({ purpose: 'login', customToken: 'synthetic-custom' })
				: Response.json(
						{ error: { code: 'WEB_ACCOUNT_REQUIRED' } },
						{ status: 409 },
					),
		)
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		await expect(
			runtime.confirm('a'.repeat(64), new AbortController().signal),
		).rejects.toMatchObject({ code: 'WEB_ACCOUNT_REQUIRED' })
		expect(
			request.mock.calls.filter(
				(call) => call[0] === '/api/auth/session/complete',
			),
		).toHaveLength(1)
		expect(runtime.memory.getSnapshot().error).toBe('LOGOUT_FAILED')
		expect(runtime.memory.getSnapshot().data).toBeNull()
		await runtime.memory.logout()
		expect(runtime.memory.getSnapshot().phase).toBe('anonymous')
		expect(signOut).toHaveBeenCalledTimes(2)
	})
	it('rejects a late confirm response before calling Firebase signIn when abort is ignored', async () => {
		const { session } = sessionFixture()
		let resolve: (value: Response) => void = () => {}
		const pending = new Promise<Response>((yes) => {
			resolve = yes
		})
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			vi.fn<typeof fetch>(async () => pending),
		)
		const controller = new AbortController()
		const done = runtime.confirm('a'.repeat(64), controller.signal)
		await settle()
		controller.abort()
		resolve(
			Response.json({ purpose: 'login', customToken: 'synthetic-custom' }),
		)
		await expect(done).rejects.toBeInstanceOf(RequestError)
		expect(session.signIn).not.toHaveBeenCalled()
	})
	it('a delayed failed session completion never signs out a newer identity', async () => {
		const { session, setUID } = sessionFixture()
		let resolve: (value: Response) => void = () => {}
		const pending = new Promise<Response>((yes) => {
			resolve = yes
		})
		const request = vi.fn<typeof fetch>(async (path) =>
			path === '/api/auth/youtube/confirm'
				? Response.json({ purpose: 'login', customToken: 'synthetic-custom' })
				: pending,
		)
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const done = runtime.confirm('a'.repeat(64), new AbortController().signal)
		await settle()
		setUID('newer-synthetic')
		resolve(
			Response.json({ error: { code: 'AUTH_REQUIRED' } }, { status: 401 }),
		)
		await expect(done).rejects.toMatchObject({ code: 'AUTH_REQUIRED' })
		expect(session.signOut).not.toHaveBeenCalled()
		expect(session.currentUID()).toBe('newer-synthetic')
	})
	it.each([
		401, 204,
	])('same-uid logout/relogin makes a delayed completion %i obsolete', async (status) => {
		vi.stubGlobal('window', new EventTarget())
		vi.stubGlobal(
			'document',
			Object.assign(new EventTarget(), { visibilityState: 'visible' }),
		)
		const { session, setUID } = sessionFixture()
		let resolve: (value: Response) => void = () => {}
		const pending = new Promise<Response>((yes) => {
			resolve = yes
		})
		let completions = 0
		const request = vi.fn<typeof fetch>(async (path) => {
			if (path === '/api/auth/youtube/confirm')
				return Response.json({
					purpose: 'login',
					customToken: 'synthetic-custom',
				})
			if (path === '/api/auth/session/complete')
				return ++completions === 1
					? pending
					: new Response(null, { status: 204 })
			return Response.json(fixture)
		})
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const dispose = runtime.mount()
		const done = runtime.confirm('a'.repeat(64), new AbortController().signal)
		await settle()
		expect(completions).toBe(1)
		setUID(null)
		setUID('synthetic')
		resolve(
			status === 204
				? new Response(null, { status: 204 })
				: Response.json({ error: { code: 'AUTH_REQUIRED' } }, { status }),
		)
		await expect(done).rejects.toMatchObject({ code: 'AUTH_REQUIRED' })
		await settle()
		expect(session.signOut).not.toHaveBeenCalled()
		expect(session.currentUID()).toBe('synthetic')
		expect(completions).toBe(2)
		expect(runtime.memory.getSnapshot().phase).toBe('authenticated')
		dispose()
	})
	it('reconciles an interrupted sign-in before reading MyPage on restored auth', async () => {
		vi.stubGlobal('window', new EventTarget())
		vi.stubGlobal(
			'document',
			Object.assign(new EventTarget(), { visibilityState: 'visible' }),
		)
		const { session, setUID } = sessionFixture()
		setUID('interrupted-synthetic')
		let resolve: (value: Response) => void = () => {}
		const pending = new Promise<Response>((yes) => {
			resolve = yes
		})
		const request = vi.fn<typeof fetch>(async (path) =>
			path === '/api/auth/session/complete' ? pending : Response.json(fixture),
		)
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const dispose = runtime.mount()
		await settle()
		expect(request.mock.calls.map(([path]) => path)).toEqual([
			'/api/auth/session/complete',
		])
		expect(runtime.memory.getSnapshot().phase).toBe('bootstrapping')
		expect(runtime.memory.getSnapshot().data).toBeNull()
		resolve(new Response(null, { status: 204 }))
		await settle()
		expect(request.mock.calls.map(([path]) => path)).toEqual([
			'/api/auth/session/complete',
			'/api/mypage',
		])
		await vi.waitFor(() =>
			expect(runtime.memory.getSnapshot().data).not.toBeNull(),
		)
		expect(session.signIn).not.toHaveBeenCalled()
		dispose()
	})
	it('restored auth retries transient completion at most three times then signs out without fetching data', async () => {
		vi.useFakeTimers()
		vi.stubGlobal('window', new EventTarget())
		vi.stubGlobal(
			'document',
			Object.assign(new EventTarget(), { visibilityState: 'visible' }),
		)
		const { session, setUID } = sessionFixture()
		setUID('interrupted-synthetic')
		const request = vi.fn<typeof fetch>(async () =>
			Response.json(
				{ error: { code: 'TEMPORARY_UNAVAILABLE' } },
				{ status: 503 },
			),
		)
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const dispose = runtime.mount()
		await vi.runAllTimersAsync()
		expect(request.mock.calls.map(([path]) => path)).toEqual(
			Array(3).fill('/api/auth/session/complete'),
		)
		expect(session.signOut).toHaveBeenCalledTimes(1)
		expect(runtime.memory.getSnapshot().data).toBeNull()
		expect(runtime.memory.getSnapshot().phase).toBe('anonymous')
		dispose()
	})
	it('pagehide aborts restored proof acquisition and a late proof never dispatches an API call', async () => {
		const windowTarget = new EventTarget()
		vi.stubGlobal('window', windowTarget)
		vi.stubGlobal(
			'document',
			Object.assign(new EventTarget(), { visibilityState: 'visible' }),
		)
		const { session, setUID } = sessionFixture()
		setUID('interrupted-synthetic')
		let resolve: (token: string) => void = () => {}
		session.appCheck = () =>
			new Promise((yes) => {
				resolve = yes
			})
		const request = vi.fn<typeof fetch>()
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const dispose = runtime.mount()
		await settle()
		windowTarget.dispatchEvent(new Event('pagehide'))
		resolve('synthetic-late-proof')
		await settle()
		expect(request).not.toHaveBeenCalled()
		expect(session.signOut).not.toHaveBeenCalled()
		expect(runtime.memory.getSnapshot().data).toBeNull()
		dispose()
	})
	it('rejects unsupported channel purposes and never treats support proof as a login token', async () => {
		const { session } = sessionFixture()
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			vi.fn<typeof fetch>(async (path) =>
				path === '/api/auth/youtube/channel'
					? Response.json({
							purpose: 'support',
							supportPurpose: 'login',
							displayName: 'Synthetic channel',
							handle: null,
							avatarUrl: null,
							confirmationRef: 'a'.repeat(64),
						})
					: Response.json({
							purpose: 'support',
							requestRef: 'b'.repeat(64),
							customToken: 'synthetic-invalid',
						}),
			),
		)
		await expect(
			runtime.channel(new AbortController().signal),
		).rejects.toMatchObject({ code: 'TEMPORARY_UNAVAILABLE' })
		await expect(
			runtime.confirm('a'.repeat(64), new AbortController().signal),
		).rejects.toMatchObject({ code: 'TEMPORARY_UNAVAILABLE' })
		expect(session.signIn).not.toHaveBeenCalled()
	})
	it('support proof returns only a reference and never signs in or completes a Firebase session', async () => {
		const { session, setUID } = sessionFixture()
		setUID('existing-synthetic')
		const request = vi.fn<typeof fetch>(async () =>
			Response.json({ purpose: 'support', requestRef: 'b'.repeat(64) }),
		)
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		await expect(
			runtime.confirm('a'.repeat(64), new AbortController().signal, 'support'),
		).resolves.toEqual({ purpose: 'support', requestRef: 'b'.repeat(64) })
		expect(request).toHaveBeenCalledTimes(1)
		expect(request.mock.calls[0]?.[0]).toBe('/api/auth/youtube/confirm')
		expect(
			new Headers(request.mock.calls[0]?.[1]?.headers).has('Authorization'),
		).toBe(false)
		expect(session.signIn).not.toHaveBeenCalled()
		expect(session.signOut).not.toHaveBeenCalled()
		expect(session.currentUID()).toBe('existing-synthetic')
	})
	it.each([
		'logout',
		'uid-change',
		'same-uid-relogin',
		'pagehide',
		'dispose',
		'replacement',
	])('discards delayed support success after %s, even when the transport ignores abort', async (change) => {
		const windowTarget = new EventTarget()
		vi.stubGlobal('window', windowTarget)
		vi.stubGlobal(
			'document',
			Object.assign(new EventTarget(), { visibilityState: 'visible' }),
		)
		const { session, setUID } = sessionFixture()
		setUID('synthetic-a')
		let release: (response: Response) => void = () => {}
		let confirmSignal: AbortSignal | null | undefined
		const request = vi.fn<typeof fetch>(async (path, init) => {
			if (path === '/api/auth/youtube/confirm') {
				confirmSignal = init?.signal
				return new Promise<Response>((resolve) => {
					release = resolve
				})
			}
			return path === '/api/auth/session/complete'
				? new Response(null, { status: 204 })
				: Response.json(fixture)
		})
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		const dispose = runtime.mount()
		await settle()
		const done = runtime.confirm(
			'a'.repeat(64),
			new AbortController().signal,
			'support',
		)
		const rejected = expect(done).rejects.toBeInstanceOf(RequestError)
		await settle()
		expect(confirmSignal?.aborted).toBe(false)
		const originalSignal = confirmSignal
		const originalRelease = release
		if (change === 'logout') await runtime.memory.logout()
		if (change === 'uid-change') setUID('synthetic-b')
		if (change === 'same-uid-relogin') {
			setUID(null)
			setUID('synthetic-a')
		}
		if (change === 'pagehide') {
			windowTarget.dispatchEvent(new Event('pagehide'))
			windowTarget.dispatchEvent(new Event('pageshow'))
		}
		if (change === 'dispose') dispose()
		if (change === 'replacement') {
			const replacement = new AbortController()
			const newer = runtime
				.confirm('c'.repeat(64), replacement.signal, 'support')
				.catch(() => {})
			replacement.abort()
			await newer
		}
		expect(originalSignal?.aborted).toBe(true)
		originalRelease(
			Response.json({ purpose: 'support', requestRef: 'b'.repeat(64) }),
		)
		await rejected
		await settle()
		expect(session.signIn).not.toHaveBeenCalled()
		if (change !== 'dispose') dispose()
	})
	it('support start sends only the opaque challenge with consent, never a browser purpose or target', async () => {
		const { session } = sessionFixture()
		const request = vi.fn<typeof fetch>(async () =>
			Response.json({
				authorizationUrl:
					'https://accounts.google.com/o/oauth2/v2/auth?state=synthetic',
			}),
		)
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			request,
		)
		await runtime.start(new AbortController().signal, 'a'.repeat(64))
		expect(JSON.parse(String(request.mock.calls[0]?.[1]?.body))).toEqual({
			supportChallenge: 'a'.repeat(64),
			privacyPolicyVersion: 'p1',
			privacyAccepted: true,
			termsVersion: 't1',
			termsAccepted: true,
		})
		await expect(
			runtime.start(new AbortController().signal, ''),
		).rejects.toMatchObject({ code: 'SUPPORT_CHALLENGE_INVALID' })
		expect(request).toHaveBeenCalledTimes(1)
	})
	it('never redirects an authorization URL to an arbitrary host', async () => {
		const { session } = sessionFixture()
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			vi.fn<typeof fetch>(async () =>
				Response.json({
					authorizationUrl: 'https://example.invalid/login',
					expiresAt: fixture.generatedAt,
				}),
			),
		)
		await expect(
			runtime.start(new AbortController().signal),
		).rejects.toMatchObject({ code: 'TEMPORARY_UNAVAILABLE' })
	})
	it('pagehide clears data and prevents delayed pageshow or auth events from restoring it', async () => {
		const windowTarget = new EventTarget()
		const documentTarget = Object.assign(new EventTarget(), {
			visibilityState: 'visible',
		})
		vi.stubGlobal('window', windowTarget)
		vi.stubGlobal('document', documentTarget)
		const { session, setUID } = sessionFixture()
		const runtime = new BrowserRuntime(
			session,
			{ privacy: 'p1', terms: 't1' },
			vi.fn<typeof fetch>(async (path) =>
				path === '/api/auth/session/complete'
					? new Response(null, { status: 204 })
					: Response.json(fixture),
			),
		)
		const dispose = runtime.mount()
		setUID('synthetic')
		await settle()
		await vi.waitFor(() =>
			expect(runtime.memory.getSnapshot().data).not.toBeNull(),
		)
		windowTarget.dispatchEvent(new Event('pagehide'))
		expect(runtime.memory.getSnapshot().data).toBeNull()
		setUID('another')
		await settle()
		expect(runtime.memory.getSnapshot().phase).toBe('bootstrapping')
		let resolve: (value: string | null) => void = () => {}
		session.recheck = () =>
			new Promise((yes) => {
				resolve = yes
			})
		windowTarget.dispatchEvent(new Event('pageshow'))
		windowTarget.dispatchEvent(new Event('pagehide'))
		resolve('another')
		await settle()
		expect(runtime.memory.getSnapshot().data).toBeNull()
		dispose()
	})
})
