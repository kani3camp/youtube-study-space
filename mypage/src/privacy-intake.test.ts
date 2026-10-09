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
