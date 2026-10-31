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
	})

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
