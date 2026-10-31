import { act, render, screen } from '@testing-library/react'
import type { User } from 'firebase/auth'
import { useEffect } from 'react'
import type { MonitorAuthorizationState } from '../lib/monitor-auth'
import MonitorAuthGate from './MonitorAuthGate'

const authMocks = vi.hoisted(() => ({
	observer: undefined as
		| ((state: MonitorAuthorizationState) => void)
		| undefined,
	unsubscribe: vi.fn(),
}))

vi.mock('../lib/monitor-auth', () => ({
	observeMonitorAuthorization: vi.fn(
		(observer: (state: MonitorAuthorizationState) => void) => {
			authMocks.observer = observer
			return authMocks.unsubscribe
		},
	),
}))

describe('MonitorAuthGate', () => {
	beforeEach(() => {
		authMocks.observer = undefined
		authMocks.unsubscribe.mockClear()
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
	})

	afterEach(() => {
		vi.unstubAllGlobals()
	})

	test('does not request a login notice while checking, authorized, or on a communication error', () => {
		render(<MonitorAuthGate>monitor</MonitorAuthGate>)
		expect(fetch).not.toHaveBeenCalled()
		act(() => authMocks.observer?.({ status: 'error' }))
		expect(fetch).not.toHaveBeenCalled()
		act(() => authMocks.observer?.({ status: 'authorized', user: {} as User }))
		expect(fetch).not.toHaveBeenCalled()
	})

	test('keeps missing configuration closed and reports it without calling alert', () => {
		const alert = vi.spyOn(window, 'alert').mockImplementation(() => undefined)
		render(<MonitorAuthGate>monitor data consumer</MonitorAuthGate>)
		act(() => authMocks.observer?.({ status: 'configuration-error' }))
		expect(screen.queryByText('monitor data consumer')).not.toBeInTheDocument()
		expect(screen.getByText('配信画面を表示できません')).toBeInTheDocument()
		expect(screen.queryByText('認証に失敗しました')).not.toBeInTheDocument()
		expect(fetch).toHaveBeenCalledExactlyOnceWith(
			'/api/monitor-auth-required',
			{
				method: 'POST',
				headers: { 'X-Monitor-Auth-Required': 'firebase-config-missing' },
				credentials: 'omit',
			},
		)
		expect(alert).not.toHaveBeenCalled()
		alert.mockRestore()
	})

	test.each(['signed-out'] as const)(
		'notifies the server once when %s, without sending credentials',
		(status) => {
			render(<MonitorAuthGate>monitor</MonitorAuthGate>)
			act(() => authMocks.observer?.({ status }))
			expect(fetch).toHaveBeenCalledExactlyOnceWith(
				'/api/monitor-auth-required',
				{
					method: 'POST',
					headers: { 'X-Monitor-Auth-Required': status },
					credentials: 'omit',
				},
			)
			act(() => authMocks.observer?.({ status: 'checking' }))
			act(() => authMocks.observer?.({ status }))
			expect(fetch).toHaveBeenCalledTimes(1)
			act(() =>
				authMocks.observer?.({ status: 'authorized', user: {} as User }),
			)
			act(() => authMocks.observer?.({ status: 'signed-out' }))
			expect(fetch).toHaveBeenCalledTimes(2)
		},
	)

	test('reports missing monitor privileges after the user signs in', () => {
		render(<MonitorAuthGate>monitor data consumer</MonitorAuthGate>)
		act(() => authMocks.observer?.({ status: 'signed-out' }))
		act(() => authMocks.observer?.({ status: 'checking' }))
		act(() =>
			authMocks.observer?.({ status: 'denied', issue: 'firestore-denied' }),
		)
		expect(fetch).toHaveBeenLastCalledWith('/api/monitor-auth-required', {
			method: 'POST',
			headers: { 'X-Monitor-Auth-Required': 'firestore-denied' },
			credentials: 'omit',
		})
		expect(fetch).toHaveBeenCalledTimes(2)
		expect(screen.queryByText('monitor data consumer')).not.toBeInTheDocument()
	})

	test('keeps the monitor closed when the notice request fails', async () => {
		vi.mocked(fetch).mockRejectedValueOnce(new Error('offline'))
		render(<MonitorAuthGate>monitor data consumer</MonitorAuthGate>)
		await act(async () => authMocks.observer?.({ status: 'signed-out' }))
		expect(screen.queryByText('monitor data consumer')).not.toBeInTheDocument()
		expect(screen.getByText('認証に失敗しました')).toBeInTheDocument()
	})

	test('mounts monitor data consumers only while authorization is valid', () => {
		const view = render(
			<MonitorAuthGate>
				<div>monitor data consumer</div>
			</MonitorAuthGate>,
		)

		expect(screen.queryByText('monitor data consumer')).not.toBeInTheDocument()

		act(() => {
			authMocks.observer?.({ status: 'signed-out' })
		})
		expect(screen.queryByText('monitor data consumer')).not.toBeInTheDocument()

		act(() => {
			authMocks.observer?.({
				status: 'authorized',
				user: {} as User,
			})
		})
		expect(screen.getByText('monitor data consumer')).toBeInTheDocument()

		act(() => {
			authMocks.observer?.({ status: 'denied', issue: 'firestore-denied' })
		})
		expect(screen.queryByText('monitor data consumer')).not.toBeInTheDocument()

		view.unmount()
		expect(authMocks.unsubscribe).toHaveBeenCalledOnce()
	})

	test('does not remount monitor data consumers for a renewed authorization', () => {
		const mounted = vi.fn()
		const unmounted = vi.fn()
		const MonitorDataConsumer = () => {
			useEffect(() => {
				mounted()
				return unmounted
			}, [])
			return <div>stateful monitor data consumer</div>
		}
		const user = { uid: 'monitor-user' } as User

		render(
			<MonitorAuthGate>
				<MonitorDataConsumer />
			</MonitorAuthGate>,
		)
		act(() => {
			authMocks.observer?.({ status: 'authorized', user })
		})
		expect(mounted).toHaveBeenCalledOnce()

		act(() => {
			authMocks.observer?.({ status: 'authorized', user })
		})
		expect(mounted).toHaveBeenCalledOnce()
		expect(unmounted).not.toHaveBeenCalled()

		act(() => {
			authMocks.observer?.({ status: 'denied', issue: 'firestore-denied' })
		})
		expect(unmounted).toHaveBeenCalledOnce()
	})
})
