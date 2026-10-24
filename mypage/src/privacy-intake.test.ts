import { expect, it, vi } from 'vitest'
import { RequestError } from './features/mypage/memory'
import { BrowserRuntime, type BrowserSession } from './runtime'

function session() {
	let uid: string | null = 'UCsynthetic'
	const value: BrowserSession = {
		currentUID: () => uid,
		idToken: async () => 'synthetic-id',
		appCheck: async () => 'synthetic-app',
		subscribe: (callback) => {
			callback(uid)
			return () => {}
		},
		recheck: async () => uid,
		signIn: async () => uid ?? '',
		signOut: async () => {
			uid = null
		},
	}
	return {
		value,
		change: (next: string | null) => {
			uid = next
		},
	}
}
const ref = 'a'.repeat(64)

it('sends only purpose, bounded body and stable submission key with authenticated credentials', async () => {
	const { value } = session()
	const request = vi.fn<typeof fetch>(async () =>
		Response.json({
			requestRef: ref,
			purpose: 'delete',
			status: 'awaiting_proof',
			acceptedAt: '2026-10-09T00:00:00Z',
			deleteBy: '2026-10-16T00:00:00Z',
			supportChallenge: 'b'.repeat(64),
		}),
	)
	const runtime = new BrowserRuntime(
		value,
		{ privacy: 'p1', terms: 't1' },
		request,
		null,
		true,
	)
	const receipt = await runtime.submitPrivacyRequest(
		'delete',
		'synthetic request',
		'synthetic-submission-key',
		new AbortController().signal,
	)
	expect(receipt.requestRef).toBe(ref)
	const [path, init] = request.mock.calls[0]
	expect(path).toBe('/api/privacy/requests')
	expect(JSON.parse(String(init?.body))).toEqual({
		purpose: 'delete',
		body: 'synthetic request',
		submissionKey: 'synthetic-submission-key',
	})
	expect(new Headers(init?.headers).get('Authorization')).toBe(
		'Bearer synthetic-id',
	)
})

it('refuses no-session and discards an old identity response', async () => {
	const { value, change } = session()
	let release: (response: Response) => void = () => {}
	const request = vi.fn<typeof fetch>(
		async () =>
			new Promise<Response>((resolve) => {
				release = resolve
			}),
	)
	const runtime = new BrowserRuntime(
		value,
		{ privacy: 'p1', terms: 't1' },
		request,
		null,
		true,
	)
	const pending = runtime.privacyRequestStatus(
		ref,
		new AbortController().signal,
	)
	await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1))
	change('UCother')
	release(
		Response.json({
			requestRef: ref,
			purpose: 'delete',
			status: 'verified',
			acceptedAt: '2026-10-09T00:00:00Z',
			deleteBy: null,
			verifiedAt: '2026-10-09T01:00:00Z',
			reply: 'private reply',
		}),
	)
	await expect(pending).rejects.toMatchObject({ code: 'AUTH_REQUIRED' })
	change(null)
	await expect(
		runtime.submitPrivacyRequest(
			'delete',
			'x',
			'synthetic-submission-key',
			new AbortController().signal,
		),
	).rejects.toMatchObject({ status: 401 })
	expect(request).toHaveBeenCalledTimes(1)
})

it('never accepts a forged reference or wrong status payload', async () => {
	const { value } = session()
	const runtime = new BrowserRuntime(
		value,
		{ privacy: 'p1', terms: 't1' },
		vi.fn<typeof fetch>(async () =>
			Response.json({
				requestRef: 'b'.repeat(64),
				status: 'verified',
				reply: 'private reply',
			}),
		),
		null,
		true,
	)
	await expect(
		runtime.privacyRequestStatus(ref, new AbortController().signal),
	).rejects.toBeInstanceOf(RequestError)
})

it('restores the server-bound receipt after navigation and uses it for reply, never the proof reference', async () => {
	const { value } = session()
	const proofRef = 'b'.repeat(64)
	let verified = false
	const request = vi.fn<typeof fetch>(async (path, init) => {
		if (path === '/api/privacy/requests')
			return Response.json({
				requestRef: ref,
				purpose: 'delete',
				status: 'awaiting_proof',
				acceptedAt: '2026-10-09T00:00:00Z',
				deleteBy: '2026-10-16T00:00:00Z',
				supportChallenge: 'c'.repeat(64),
			})
		if (path === '/api/auth/youtube/confirm') {
			verified = true
			return Response.json({
				purpose: 'support',
				requestRef: proofRef,
				supportRequestRef: ref,
			})
		}
		if (path === '/api/privacy/requests/status') {
			const body = JSON.parse(String(init?.body)) as { requestRef: string }
			if (!verified || body.requestRef !== ref)
				return Response.json(
					{ error: { code: 'SUPPORT_CHALLENGE_INVALID' } },
					{ status: 400 },
				)
			return Response.json({
				requestRef: ref,
				purpose: 'delete',
				status: 'verified',
				acceptedAt: '2026-10-09T00:00:00Z',
				deleteBy: '2026-10-16T00:00:00Z',
				verifiedAt: '2026-10-09T01:00:00Z',
				reply: 'Synthetic reply',
			})
		}
		throw new Error('unexpected route')
	})
	const firstPage = new BrowserRuntime(
		value,
		{ privacy: 'p1', terms: 't1' },
		request,
		null,
		true,
	)
	const intake = await firstPage.submitPrivacyRequest(
		'delete',
		'',
		'synthetic-submission-key',
		new AbortController().signal,
	)
	expect(intake.requestRef).toBe(ref)
	const afterOAuthNavigation = new BrowserRuntime(
		value,
		{ privacy: 'p1', terms: 't1' },
		request,
		null,
		true,
	)
	const confirmation = await afterOAuthNavigation.confirm(
		'd'.repeat(64),
		new AbortController().signal,
		'support',
	)
	expect(confirmation).toEqual({
		purpose: 'support',
		requestRef: proofRef,
		supportRequestRef: ref,
	})
	if (confirmation.purpose !== 'support' || !confirmation.supportRequestRef)
		throw new Error('missing bound request ref')
	await expect(
		afterOAuthNavigation.privacyRequestStatus(
			confirmation.requestRef,
			new AbortController().signal,
		),
	).rejects.toMatchObject({ code: 'SUPPORT_CHALLENGE_INVALID' })
	await expect(
		afterOAuthNavigation.privacyRequestStatus(
			confirmation.supportRequestRef,
			new AbortController().signal,
		),
	).resolves.toMatchObject({ reply: 'Synthetic reply' })
	expect(request.mock.calls.map(([path]) => path)).toEqual([
		'/api/privacy/requests',
		'/api/auth/youtube/confirm',
		'/api/privacy/requests/status',
		'/api/privacy/requests/status',
	])
})
