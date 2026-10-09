// Dev-only factory injection. The product entrypoint never imports this module
// and never switches to a mock based on its own URL, query or persisted flags.
import { createMemoryHistory, RouterProvider } from '@tanstack/react-router'
import { createRoot } from 'react-dom/client'
import fixture from '../../docs/mypage/fixtures/available.json'
import { createApp } from './app'
import type { MyPage } from './features/mypage/contract'
import { BrowserRuntime, type BrowserSession } from './runtime'
import './style.css'

if (import.meta.env.DEV) {
	const search = new URLSearchParams(location.search)
	const mode = search.get('mode')
	let uid: string | null =
		mode?.includes('authenticated') || mode === 'late-response'
			? 'synthetic-a'
			: null
	let logoutFails = mode === 'logout-failure'
	let restriction: string | null = mode?.includes('restricted-deletion')
		? 'DATA_DELETION_IN_PROGRESS'
		: mode?.includes('restricted-moderation')
			? 'SERVICE_ACCESS_RESTRICTED'
			: null
	const requests: string[] = []
	let holdNext = false
	let release: (() => void) | null = null
	const listeners = new Set<(uid: string | null) => void>()
	const emit = (next: string | null) => {
		uid = next
		for (const listener of listeners) listener(next)
	}
	const session: BrowserSession = {
		currentUID: () => uid,
		idToken: async (wanted) => {
			if (wanted !== uid) throw new Error('synthetic identity change')
			return `synthetic-id:${uid}`
		},
		appCheck: async () => 'synthetic-app',
		subscribe: (listener) => {
			listeners.add(listener)
			const timer = setTimeout(() => listener(uid), 20)
			return () => {
				clearTimeout(timer)
				listeners.delete(listener)
			}
		},
		recheck: async () => uid,
		signIn: async () => {
			if (mode?.startsWith('support-'))
				throw new Error('support must never call signIn')
			emit('synthetic-a')
			return 'synthetic-a'
		},
		signOut: async () => {
			if (logoutFails) {
				logoutFails = false
				throw new Error('synthetic signout failure')
			}
			emit(null)
		},
	}
	const request: typeof fetch = async (path, init) => {
		requests.push(String(path))
		const counter = document.getElementById('synthetic-requests')
		if (counter) counter.textContent = JSON.stringify(requests)
		if (
			mode === 'privacy-intake-authenticated' &&
			path === '/api/privacy/requests'
		) {
			if (holdNext) {
				holdNext = false
				await new Promise<void>((resolve) => {
					release = resolve
				})
			}
			return Response.json({
				requestRef: 'c'.repeat(64),
				supportChallenge: 'd'.repeat(64),
				purpose: 'delete',
				status: 'awaiting_proof',
				acceptedAt: '2026-10-09T00:00:00Z',
				deleteBy: '2026-10-16T00:00:00Z',
			})
		}
		if (
			mode === 'privacy-intake-authenticated' &&
			path === '/api/privacy/requests/status'
		)
			return Response.json({
				requestRef: 'c'.repeat(64),
				purpose: 'delete',
				status: 'verified',
				acceptedAt: '2026-10-09T00:00:00Z',
				deleteBy: '2026-10-16T00:00:00Z',
				verifiedAt: '2026-10-09T01:00:00Z',
				reply: 'Synthetic operator response',
			})
		if (
			restriction &&
			((path === '/api/auth/session/complete' &&
				(!mode?.includes('confirm') || mode?.startsWith('support-confirm-'))) ||
				(path === '/api/auth/youtube/confirm' &&
					mode?.includes('confirm') &&
					!mode?.startsWith('support-confirm-')) ||
				path === '/api/mypage')
		)
			return Response.json({ error: { code: restriction } }, { status: 403 })
		if (path === '/api/auth/youtube/channel' && mode === 'support-mismatch')
			return Response.json(
				{ error: { code: 'SUPPORT_CHANNEL_MISMATCH' } },
				{ status: 400 },
			)
		if (path === '/api/auth/youtube/channel' && mode === 'missing-channel')
			return Response.json(
				{ error: { code: 'OAUTH_TRANSACTION_REQUIRED' } },
				{ status: 400 },
			)
		if (path === '/api/auth/youtube/channel')
			return Response.json({
				...(mode?.startsWith('support-confirm-')
					? { purpose: 'support', supportPurpose: mode.split('-')[2] }
					: { purpose: 'login' }),
				displayName: 'Sample Channel',
				handle: '@sample',
				avatarUrl:
					mode === 'channel-avatar-failure'
						? 'https://invalid.example/synthetic-avatar.png'
						: null,
				confirmationRef: 'a'.repeat(64),
			})
		if (path === '/api/auth/youtube/confirm') {
			if (holdNext) {
				holdNext = false
				await new Promise<void>((resolve) => {
					release = resolve
				})
			}
			return Response.json(
				mode?.startsWith('support-confirm-')
					? { purpose: 'support', requestRef: 'b'.repeat(64) }
					: { purpose: 'login', customToken: 'synthetic-custom' },
			)
		}
		if (path === '/api/auth/session/complete' && mode?.startsWith('support-'))
			throw new Error('support must never complete a session')
		if (path === '/api/auth/session/complete')
			return new Response(null, { status: 204 })
		if (path === '/api/auth/youtube/start')
			return Response.json({
				authorizationUrl:
					'https://accounts.google.com/o/oauth2/v2/auth?state=synthetic',
				expiresAt: fixture.generatedAt,
			})
		if (path !== '/api/mypage')
			return Response.json(
				{ error: { code: 'INVALID_REQUEST' } },
				{ status: 404 },
			)
		const response = structuredClone(fixture) as MyPage
		const token = new Headers(init?.headers).get('Authorization')
		if (token?.includes('synthetic-b') && response.current.data)
			response.current.data.workName = '新しい合成作業'
		if (holdNext) {
			holdNext = false
			await new Promise<void>((resolve) => {
				release = resolve
			})
		}
		return Response.json(response)
	}
	const runtime = new BrowserRuntime(
		session,
		{ privacy: 'synthetic-p1', terms: 'synthetic-t1' },
		request,
		null,
		mode?.startsWith('privacy-intake') ?? false,
	)
	const initial = search.get('path') ?? '/'
	const router = createApp(
		runtime,
		createMemoryHistory({ initialEntries: [initial] }),
	)
	const root = document.getElementById('root')
	if (root)
		createRoot(root).render(
			<>
				<RouterProvider router={router} />
				<aside
					className="synthetic-controls"
					aria-label="Synthetic test controls"
				>
					<output id="synthetic-requests" className="sr-only">
						[]
					</output>
					<button
						id="synthetic-restrict-moderation"
						type="button"
						onClick={() => {
							restriction = 'SERVICE_ACCESS_RESTRICTED'
							void runtime.memory.refresh()
						}}
					>
						Restrict synthetic moderation
					</button>
					<button
						id="synthetic-restrict-deletion"
						type="button"
						onClick={() => {
							restriction = 'DATA_DELETION_IN_PROGRESS'
							void runtime.memory.refresh()
						}}
					>
						Restrict synthetic deletion
					</button>
					<button
						id="synthetic-clear-restriction"
						type="button"
						onClick={() => {
							restriction = null
						}}
					>
						Clear synthetic restriction
					</button>
					<button
						id="synthetic-hold"
						type="button"
						onClick={() => {
							holdNext = true
						}}
					>
						Hold response
					</button>
					<button
						id="synthetic-switch"
						type="button"
						onClick={() => emit('synthetic-b')}
					>
						Change synthetic uid
					</button>
					<button
						id="synthetic-logout"
						type="button"
						onClick={() => emit(null)}
					>
						Logout synthetic uid
					</button>
					<button
						id="synthetic-release"
						type="button"
						onClick={() => release?.()}
					>
						Release response
					</button>
					<button
						id="synthetic-fail-logout"
						type="button"
						onClick={() => {
							logoutFails = true
						}}
					>
						Fail next signout
					</button>
				</aside>
			</>,
		)
}
