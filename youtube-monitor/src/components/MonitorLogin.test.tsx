import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { User } from 'firebase/auth'
import type { MonitorAuthorizationState } from '../lib/monitor-auth'
import MonitorLogin from '../pages/monitor-login'

const mocks = vi.hoisted(() => ({
	signIn: vi.fn(),
	replace: vi.fn(),
	checkAccess: vi.fn(),
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
vi.mock('../lib/monitor-firestore-access', () => ({
	checkMonitorFirestoreAccess: mocks.checkAccess,
}))

describe('monitor operator login', () => {
	beforeEach(() => {
		mocks.signIn.mockReset().mockResolvedValue(undefined)
		mocks.replace.mockReset()
		mocks.observer = undefined
		mocks.checkAccess.mockReset().mockResolvedValue('ready')
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
			expect(mocks.checkAccess).not.toHaveBeenCalled()
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
		expect(mocks.checkAccess).not.toHaveBeenCalled()
	})

	test.each(['authorized', 'unauthorized'] as const)(
		'shows an existing login as successful when monitor authorization is %s',
		(status) => {
			render(<MonitorLogin />)
			act(() => {
				mocks.observer?.(
					status === 'authorized' ? { status, user: {} as User } : { status },
				)
			})
			expect(screen.getByRole('status')).toHaveTextContent('ログインしました。')
			expect(screen.queryByLabelText('パスワード')).not.toBeInTheDocument()
			expect(mocks.replace).not.toHaveBeenCalled()
		},
	)

	test('shows and logs a missing claim without reading Firestore', async () => {
		render(<MonitorLogin />)
		act(() => mocks.observer?.({ status: 'unauthorized' }))
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'monitor_reader: true',
		)
		expect(mocks.checkAccess).not.toHaveBeenCalled()
		expect(fetch).toHaveBeenCalledExactlyOnceWith(
			'/api/monitor-auth-required',
			{
				method: 'POST',
				headers: { 'X-Monitor-Auth-Required': 'unauthorized' },
				credentials: 'omit',
			},
		)
	})

	test.each([
		['firestore-denied', '_monitor-readers/{UID}', 'Security Rules'],
		['firestore-config-missing', 'config/constants', '見つかりません'],
		['firestore-error', 'ネットワーク', '接続を確認できません'],
	])(
		'shows and logs %s while preserving login success',
		async (issue, detail, message) => {
			mocks.checkAccess.mockResolvedValue(issue)
			render(<MonitorLogin />)
			act(() => mocks.observer?.({ status: 'authorized', user: {} as User }))
			expect(
				screen.getByText('モニター閲覧設定を確認しています。'),
			).toBeInTheDocument()
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
		expect(mocks.checkAccess).toHaveBeenCalledOnce()
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
		expect(fetch).not.toHaveBeenCalled()
	})

	test('ignores a pending result after the account signs out', async () => {
		let finish: (result: string) => void = () => undefined
		mocks.checkAccess.mockReturnValue(
			new Promise((resolve) => {
				finish = resolve
			}),
		)
		render(<MonitorLogin />)
		act(() => mocks.observer?.({ status: 'authorized', user: {} as User }))
		act(() => mocks.observer?.({ status: 'signed-out' }))
		await act(async () => finish('firestore-denied'))
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
		expect(screen.getByLabelText('メールアドレス')).toBeInTheDocument()
		expect(fetch).not.toHaveBeenCalled()
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
