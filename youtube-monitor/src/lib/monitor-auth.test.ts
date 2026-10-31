import type { User } from 'firebase/auth'

const authMocks = vi.hoisted(() => ({
	auth: {},
	onIdTokenChanged: vi.fn(),
	setPersistence: vi.fn(),
	tokenObserver: undefined as ((user: User | null) => void) | undefined,
}))

vi.mock('firebase/auth', () => ({
	browserLocalPersistence: {},
	getAuth: vi.fn(() => authMocks.auth),
	onIdTokenChanged: authMocks.onIdTokenChanged,
	setPersistence: authMocks.setPersistence,
	signInWithEmailAndPassword: vi.fn(),
	signOut: vi.fn(),
}))

vi.mock('./firestore', () => ({
	getFirebaseApp: vi.fn(() => ({})),
}))

import {
	hasMonitorReaderClaim,
	observeMonitorAuthorization,
} from './monitor-auth'

vi.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: vi.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: vi.fn(() => ({
		style: { fontFamily: 'mock-source-code-pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

const userWithClaim = (monitorReader: unknown, uid = 'monitor-user'): User =>
	({
		uid,
		getIdTokenResult: vi.fn().mockResolvedValue({
			claims: { monitor_reader: monitorReader },
		}),
	}) as unknown as User

describe('hasMonitorReaderClaim', () => {
	test('accepts only the dedicated boolean monitor claim', async () => {
		await expect(hasMonitorReaderClaim(userWithClaim(true))).resolves.toBe(true)
	})

	test.each([false, undefined, 'true', 1])(
		'rejects a non-boolean-true claim (%s)',
		async (claim) => {
			await expect(hasMonitorReaderClaim(userWithClaim(claim))).resolves.toBe(
				false,
			)
		},
	)
})

describe('observeMonitorAuthorization', () => {
	beforeEach(() => {
		authMocks.tokenObserver = undefined
		authMocks.setPersistence.mockReset().mockResolvedValue(undefined)
		authMocks.onIdTokenChanged
			.mockReset()
			.mockImplementation(
				(_auth: unknown, next: (user: User | null) => void): (() => void) => {
					authMocks.tokenObserver = next
					return vi.fn()
				},
			)
	})

	test('keeps an authorized monitor mounted during a same-user token refresh', async () => {
		const observer = vi.fn()
		const user = userWithClaim(true)
		const stop = observeMonitorAuthorization(observer)

		await vi.waitFor(() => {
			expect(authMocks.tokenObserver).toBeDefined()
		})
		authMocks.tokenObserver?.(user)
		await vi.waitFor(() => {
			expect(observer).toHaveBeenLastCalledWith({
				status: 'authorized',
				user,
			})
		})

		observer.mockClear()
		authMocks.tokenObserver?.(user)
		await vi.waitFor(() => {
			expect(observer).toHaveBeenCalledWith({ status: 'authorized', user })
		})
		expect(observer).not.toHaveBeenCalledWith({ status: 'checking' })

		stop()
	})

	test('still fails closed when a refreshed claim is revoked', async () => {
		const observer = vi.fn()
		const user = userWithClaim(true)
		const getIdTokenResult = vi.mocked(user.getIdTokenResult)
		const stop = observeMonitorAuthorization(observer)

		await vi.waitFor(() => {
			expect(authMocks.tokenObserver).toBeDefined()
		})
		authMocks.tokenObserver?.(user)
		await vi.waitFor(() => {
			expect(observer).toHaveBeenLastCalledWith({
				status: 'authorized',
				user,
			})
		})

		getIdTokenResult.mockResolvedValueOnce({
			claims: { monitor_reader: false },
		} as unknown as Awaited<ReturnType<User['getIdTokenResult']>>)
		observer.mockClear()
		authMocks.tokenObserver?.(user)
		await vi.waitFor(() => {
			expect(observer).toHaveBeenLastCalledWith({ status: 'unauthorized' })
		})
		expect(observer).not.toHaveBeenCalledWith({ status: 'checking' })

		stop()
	})

	test('returns to checking before authorizing a different user', async () => {
		const observer = vi.fn()
		const firstUser = userWithClaim(true, 'first-monitor-user')
		const secondUser = userWithClaim(false, 'second-monitor-user')
		const stop = observeMonitorAuthorization(observer)

		await vi.waitFor(() => {
			expect(authMocks.tokenObserver).toBeDefined()
		})
		authMocks.tokenObserver?.(firstUser)
		await vi.waitFor(() => {
			expect(observer).toHaveBeenLastCalledWith({
				status: 'authorized',
				user: firstUser,
			})
		})

		observer.mockClear()
		authMocks.tokenObserver?.(secondUser)
		expect(observer).toHaveBeenLastCalledWith({ status: 'checking' })
		await vi.waitFor(() => {
			expect(observer).toHaveBeenLastCalledWith({ status: 'unauthorized' })
		})

		stop()
	})

	test('ignores an obsolete claim result after sign-out', async () => {
		let resolveClaim: ((value: boolean) => void) | undefined
		const claimResult = new Promise<boolean>((resolve) => {
			resolveClaim = resolve
		})
		const user = {
			uid: 'monitor-user',
			getIdTokenResult: vi.fn(async () => ({
				claims: { monitor_reader: await claimResult },
			})),
		} as unknown as User
		const observer = vi.fn()
		const stop = observeMonitorAuthorization(observer)

		await vi.waitFor(() => {
			expect(authMocks.tokenObserver).toBeDefined()
		})
		authMocks.tokenObserver?.(user)
		expect(observer).toHaveBeenLastCalledWith({ status: 'checking' })
		authMocks.tokenObserver?.(null)
		expect(observer).toHaveBeenLastCalledWith({ status: 'signed-out' })

		resolveClaim?.(true)
		await claimResult
		await Promise.resolve()
		expect(observer).not.toHaveBeenCalledWith({ status: 'authorized', user })
		expect(observer).toHaveBeenLastCalledWith({ status: 'signed-out' })

		stop()
	})
})
