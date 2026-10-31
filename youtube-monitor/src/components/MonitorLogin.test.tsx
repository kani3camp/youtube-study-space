import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { User } from 'firebase/auth'
import type { MonitorAuthorizationState } from '../lib/monitor-auth'
import MonitorLogin from '../pages/monitor-login'

const mocks = vi.hoisted(() => ({
	signIn: vi.fn(),
	replace: vi.fn(),
	observer: undefined as
		| ((state: MonitorAuthorizationState) => void)
		| undefined,
}))
vi.mock('next/router', () => ({
	useRouter: () => ({ replace: mocks.replace }),
}))
vi.mock('../lib/monitor-auth', () => ({
	observeMonitorAuthorization: (
		observer: (state: MonitorAuthorizationState) => void,
	) => {
		mocks.observer = observer
		return () => undefined
	},
	signInMonitor: mocks.signIn,
}))

describe('monitor operator login', () => {
	beforeEach(() => {
		mocks.signIn.mockReset().mockResolvedValue(undefined)
		mocks.replace.mockReset()
		mocks.observer = undefined
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
	})
	afterEach(() => vi.unstubAllGlobals())

	const submit = async () => {
		const user = userEvent.setup()
		await user.type(
			screen.getByLabelText('メールアドレス'),
			'monitor@example.test',
		)
		await user.type(
			screen.getByLabelText('パスワード'),
			'synthetic-test-password',
		)
		await user.click(screen.getByRole('button', { name: 'ログイン' }))
	}

	test.each(['signed-out', 'checking', 'error'] as const)(
		'does not probe Firestore while authentication is %s',
		(status) => {
			render(<MonitorLogin />)
			act(() => mocks.observer?.({ status }))
			expect(fetch).not.toHaveBeenCalled()
		},
	)

	test('shows success without navigating after password login, even before monitor authorization resolves', async () => {
		render(<MonitorLogin />)
		await submit()
		expect(mocks.signIn).toHaveBeenCalledWith(
			'monitor@example.test',
			'synthetic-test-password',
		)
		expect(await screen.findByRole('status')).toHaveTextContent(
			'ログインしました。',
		)
		expect(screen.queryByLabelText('パスワード')).not.toBeInTheDocument()
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
		expect(mocks.replace).not.toHaveBeenCalled()
	})

	test('shows an existing session as successful when Firestore access is allowed', () => {
		render(<MonitorLogin />)
		act(() => mocks.observer?.({ status: 'authorized', user: {} as User }))
		expect(screen.getByRole('status')).toHaveTextContent('ログインしました。')
		expect(screen.queryByLabelText('パスワード')).not.toBeInTheDocument()
		expect(mocks.replace).not.toHaveBeenCalled()
	})

	test.each([
		['firestore-denied', '_monitor-readers/{UID}', 'Security Rules'],
		['firestore-config-missing', 'config/constants', '見つかりません'],
		['firestore-error', 'ネットワーク', '接続を確認できません'],
	] as const)(
		'shows and logs %s while preserving login success',
		async (issue, detail, message) => {
			render(<MonitorLogin />)
			act(() => mocks.observer?.({ status: 'denied', issue }))
			expect(await screen.findByRole('alert')).toHaveTextContent(detail)
			expect(screen.getByRole('alert')).toHaveTextContent(message)
			expect(screen.getByRole('status')).toHaveTextContent('ログインしました。')
			expect(fetch).toHaveBeenCalledExactlyOnceWith(
				'/api/monitor-auth-required',
				{
					method: 'POST',
					headers: { 'X-Monitor-Auth-Required': issue },
					credentials: 'omit',
				},
			)
			expect(mocks.replace).not.toHaveBeenCalled()
		},
	)

	test('does not report an issue when Firestore access succeeds', async () => {
		render(<MonitorLogin />)
		await act(async () =>
			mocks.observer?.({ status: 'authorized', user: {} as User }),
		)
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
		expect(fetch).not.toHaveBeenCalled()
	})

	test('explains required Firebase settings without presenting a login failure', () => {
		render(<MonitorLogin />)
		act(() => mocks.observer?.({ status: 'configuration-error' }))
		expect(screen.getByRole('alert')).toHaveTextContent(
			'Firebaseの接続設定が不足',
		)
		expect(screen.getByRole('alert')).toHaveTextContent(
			'NEXT_PUBLIC_FIREBASE_PROJECT_ID',
		)
		expect(screen.getByRole('alert')).toHaveTextContent(
			'NEXT_PUBLIC_FIREBASE_API_KEY',
		)
		expect(screen.getByRole('alert')).not.toHaveTextContent(
			'認証に失敗しました',
		)
		expect(screen.queryByLabelText('パスワード')).not.toBeInTheDocument()
		expect(fetch).toHaveBeenCalledExactlyOnceWith(
			'/api/monitor-auth-required',
			{
				method: 'POST',
				headers: { 'X-Monitor-Auth-Required': 'firebase-config-missing' },
				credentials: 'omit',
			},
		)
	})

	test('keeps the form open and does not expose raw errors when login fails', async () => {
		mocks.signIn.mockRejectedValueOnce({
			code: 'auth/invalid-credential',
			message: 'sensitive raw error',
		})
		render(<MonitorLogin />)
		await submit()
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'認証に失敗しました。',
		)
		expect(screen.getByRole('alert')).not.toHaveTextContent(
			'sensitive raw error',
		)
		expect(screen.queryByRole('status')).not.toBeInTheDocument()
		expect(screen.getByLabelText('パスワード')).toBeInTheDocument()
		expect(mocks.replace).not.toHaveBeenCalled()
	})
})
