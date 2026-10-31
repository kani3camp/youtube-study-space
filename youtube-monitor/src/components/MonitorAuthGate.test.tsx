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
			authMocks.observer?.({ status: 'unauthorized' })
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
			authMocks.observer?.({ status: 'unauthorized' })
		})
		expect(unmounted).toHaveBeenCalledOnce()
	})
})
