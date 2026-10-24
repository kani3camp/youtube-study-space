// Test-only identity injection. Product entrypoints never import this module.
// All /api requests use native fetch and the real Go HTTP handler.
import { RouterProvider } from '@tanstack/react-router'
import { createRoot } from 'react-dom/client'
import { createApp } from './app'
import { BrowserRuntime, type BrowserSession } from './runtime'
import './style.css'

if (import.meta.env.DEV) {
	const bootstrap = (await fetch('/_qa/bootstrap')).json() as Promise<{
		uid: string
		appCheck: string
	}>
	const fixture = await bootstrap
	let uid: string | null = null
	const listeners = new Set<(value: string | null) => void>()
	const emit = (value: string | null) => {
		uid = value
		for (const listener of listeners) listener(value)
	}
	const session: BrowserSession = {
		currentUID: () => uid,
		idToken: async (wanted) => {
			if (wanted !== uid || !uid) throw new Error('Synthetic identity missing')
			return 'synthetic-browser-id-token'
		},
		appCheck: async () => fixture.appCheck,
		recheck: async () => uid,
		subscribe: (listener) => {
			listeners.add(listener)
			queueMicrotask(() => listener(uid))
			return () => listeners.delete(listener)
		},
		signIn: async (token) => {
			if (token !== 'synthetic-custom-token')
				throw new Error('Synthetic custom token rejected')
			emit(fixture.uid)
			return fixture.uid
		},
		signOut: async () => emit(null),
	}
	const runtime = new BrowserRuntime(session, {
		privacy: 'synthetic-p',
		terms: 'synthetic-t',
	})
	const probe = async (headers: Record<string, string> = {}) =>
		fetch('/api/mypage', {
			headers: {
				Authorization: 'Bearer synthetic-browser-id-token',
				'X-Firebase-AppCheck': fixture.appCheck,
				...headers,
			},
		})
	Object.assign(window, {
		httpQA: {
			probe: async (headers?: Record<string, string>) => {
				const response = await probe(headers)
				return {
					status: response.status,
					retryAfter: response.headers.get('Retry-After'),
					body: await response.json(),
				}
			},
		},
	})
	const root = document.getElementById('root')
	if (!root) throw new Error('Synthetic root unavailable')
	createRoot(root).render(<RouterProvider router={createApp(runtime)} />)
}
