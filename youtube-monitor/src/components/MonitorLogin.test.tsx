import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { User } from 'firebase/auth'
import type { MonitorAuthorizationState } from '../lib/monitor-auth'
import MonitorLogin from '../pages/monitor-login'

const mocks = vi.hoisted(() => ({
	signIn: vi.fn(),
	signOut: vi.fn(),
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
		observer({ status: 'signed-out' })
		return () => undefined
	},
	signInMonitor: mocks.signIn,
	signOutMonitor: mocks.signOut,
}))

describe('monitor operator login', () => {
	beforeEach(() => {
		mocks.signIn.mockReset().mockImplementation(async () => {
			mocks.observer?.({
				status: 'authorized',
				user: { uid: 'operator' } as User,
			})
		})
		mocks.signOut.mockReset().mockImplementation(async () => {
			mocks.observer?.({ status: 'signed-out' })
		})
		mocks.replace.mockReset()
		mocks.observer = undefined
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
	})
	afterEach(() => vi.unstubAllGlobals())

	const submit = async (
		email = 'monitor@example.test',
		password = 'synthetic-test-password',
	) => {
		const user = userEvent.setup()
		await user.type(screen.getByLabelText('メールアドレス'), email)
		await user.type(screen.getByLabelText('パスワード'), password)
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

	test('waits for server authorization after password login and then shows success without navigating', async () => {
		mocks.signIn.mockImplementationOnce(async () =>
			mocks.observer?.({ status: 'checking' }),
		)
		render(<MonitorLogin />)
		await submit()
		expect(mocks.signIn).toHaveBeenCalledWith(
			'monitor@example.test',
			'synthetic-test-password',
		)
		expect(screen.getByRole('status')).toHaveTextContent('確認しています')
		expect(screen.queryByText('ログインしました。')).not.toBeInTheDocument()
		act(() => mocks.observer?.({ status: 'authorized', user: {} as User }))
		expect(screen.getByRole('status')).toHaveTextContent('ログインしました。')
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
		'shows and logs %s with an account switch action instead of success',
		async (issue, detail, message) => {
			render(<MonitorLogin />)
			act(() => mocks.observer?.({ status: 'denied', issue }))
			expect(await screen.findByRole('alert')).toHaveTextContent(detail)
			expect(screen.getByRole('alert')).toHaveTextContent(message)
			expect(screen.queryByText('ログインしました。')).not.toBeInTheDocument()
			expect(
				screen.getByRole('button', { name: '別のアカウントでログイン' }),
			).toBeInTheDocument()
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

	test('signs out the denied account, clears its inputs, and allows another account to log in', async () => {
		const user = userEvent.setup()
		render(<MonitorLogin />)
		await submit()
		act(() => mocks.observer?.({ status: 'denied', issue: 'firestore-denied' }))
		expect(screen.queryByText('ログインしました。')).not.toBeInTheDocument()
		await user.click(
			screen.getByRole('button', { name: '別のアカウントでログイン' }),
		)
		expect(mocks.signOut).toHaveBeenCalledOnce()
		expect(await screen.findByLabelText('メールアドレス')).toHaveValue('')
		expect(screen.getByLabelText('パスワード')).toHaveValue('')
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
		await submit('correct-operator@example.test', 'different-test-password')
		expect(mocks.signIn).toHaveBeenLastCalledWith(
			'correct-operator@example.test',
			'different-test-password',
		)
		expect(screen.getByRole('status')).toHaveTextContent('ログインしました。')
		expect(mocks.replace).not.toHaveBeenCalled()
	})

	test('does not expose the form until sign-out completes, even if the session observer clears first', async () => {
		let finish: () => void = () => undefined
		mocks.signOut.mockReturnValueOnce(
			new Promise<void>((resolve) => {
				finish = resolve
			}),
		)
		render(<MonitorLogin />)
		act(() => mocks.observer?.({ status: 'denied', issue: 'firestore-denied' }))
		await userEvent
			.setup()
			.click(screen.getByRole('button', { name: '別のアカウントでログイン' }))
		act(() => mocks.observer?.({ status: 'signed-out' }))
		expect(screen.getByRole('status')).toHaveTextContent('ログアウトしています')
		expect(screen.queryByLabelText('メールアドレス')).not.toBeInTheDocument()
		await act(async () => finish())
		expect(screen.getByLabelText('メールアドレス')).toHaveValue('')
	})

	test('keeps the denied account visible and allows retry when sign-out fails', async () => {
		mocks.signOut.mockRejectedValueOnce(
			new Error('sensitive raw sign-out error'),
		)
		render(<MonitorLogin />)
		act(() => mocks.observer?.({ status: 'denied', issue: 'firestore-denied' }))
		const user = userEvent.setup()
		await user.click(
			screen.getByRole('button', { name: '別のアカウントでログイン' }),
		)
		expect(
			await screen.findByText(
				'ログアウトできませんでした。もう一度お試しください。',
			),
		).toBeInTheDocument()
		expect(
			screen.queryByText('sensitive raw sign-out error'),
		).not.toBeInTheDocument()
		expect(screen.queryByLabelText('パスワード')).not.toBeInTheDocument()
		await user.click(
			screen.getByRole('button', { name: '別のアカウントでログイン' }),
		)
		expect(mocks.signOut).toHaveBeenCalledTimes(2)
		expect(await screen.findByLabelText('メールアドレス')).toBeInTheDocument()
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	})

	test.each(['signed-out', 'checking', 'error'] as const)(
		'does not keep success after the session changes to %s',
		async (status) => {
			render(<MonitorLogin />)
			await submit()
			expect(screen.getByRole('status')).toHaveTextContent('ログインしました。')
			act(() => mocks.observer?.({ status }))
			expect(screen.queryByText('ログインしました。')).not.toBeInTheDocument()
			if (status !== 'checking')
				expect(screen.getByLabelText('メールアドレス')).toBeInTheDocument()
		},
	)

	test('ignores an old password sign-in completion after the session signs out', async () => {
		let finish: () => void = () => undefined
		mocks.signIn.mockReturnValueOnce(
			new Promise<void>((resolve) => {
				finish = resolve
			}),
		)
		render(<MonitorLogin />)
		await submit()
		act(() => mocks.observer?.({ status: 'checking' }))
		act(() => mocks.observer?.({ status: 'signed-out' }))
		await act(async () => finish())
		expect(screen.queryByText('ログインしました。')).not.toBeInTheDocument()
		expect(screen.getByRole('button', { name: 'ログイン' })).toBeEnabled()
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
