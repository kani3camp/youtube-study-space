import type { User } from 'firebase/auth'

const mocks = vi.hoisted(() => ({
	auth: {},
	getAuth: vi.fn(),
	onIdTokenChanged: vi.fn(),
	setPersistence: vi.fn(),
	checkAccess: vi.fn(),
	unsubscribe: vi.fn(),
	tokenObserver: undefined as ((user: User | null) => void) | undefined,
	errorObserver: undefined as (() => void) | undefined,
}))
vi.mock('firebase/auth', () => ({
	browserLocalPersistence: {},
	getAuth: mocks.getAuth,
	onIdTokenChanged: mocks.onIdTokenChanged,
	setPersistence: mocks.setPersistence,
	signInWithEmailAndPassword: vi.fn(),
	signOut: vi.fn(),
}))
vi.mock('./firestore', () => ({ getFirebaseApp: () => ({}) }))
vi.mock('./monitor-firestore-access', () => ({
	checkMonitorFirestoreAccess: mocks.checkAccess,
}))

import { FirebaseConfigurationError } from './firebase-config'
import { observeMonitorAuthorization } from './monitor-auth'

const user = (uid = 'monitor-user') => ({ uid }) as User
const subscribed = async () => {
	await vi.waitFor(() => expect(mocks.tokenObserver).toBeDefined())
}

describe('monitor session and Firestore authorization', () => {
	beforeEach(() => {
		mocks.tokenObserver = undefined
		mocks.errorObserver = undefined
		mocks.getAuth.mockReset().mockReturnValue(mocks.auth)
		mocks.setPersistence.mockReset().mockResolvedValue(undefined)
		mocks.checkAccess.mockReset().mockResolvedValue('ready')
		mocks.unsubscribe.mockReset()
		mocks.onIdTokenChanged
			.mockReset()
			.mockImplementation(
				(
					_auth: unknown,
					next: (user: User | null) => void,
					error: () => void,
				) => {
					mocks.tokenObserver = next
					mocks.errorObserver = error
					return mocks.unsubscribe
				},
			)
	})

	test('reports Auth initialization failures without accessing Firestore', async () => {
		mocks.getAuth.mockImplementationOnce(() => {
			throw new Error('invalid-api-key')
		})
		const observer = vi.fn()
		const stop = observeMonitorAuthorization(observer)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({ status: 'error' }),
		)
		expect(mocks.checkAccess).not.toHaveBeenCalled()
		stop()
	})

	test('reports missing Firebase configuration separately from authentication failure', async () => {
		mocks.getAuth.mockImplementationOnce(() => {
			throw new FirebaseConfigurationError('NEXT_PUBLIC_FIREBASE_PROJECT_ID')
		})
		const observer = vi.fn()
		const stop = observeMonitorAuthorization(observer)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({
				status: 'configuration-error',
			}),
		)
		expect(mocks.onIdTokenChanged).not.toHaveBeenCalled()
		expect(mocks.checkAccess).not.toHaveBeenCalled()
		stop()
	})

	test('does not subscribe or access Firestore when persistence fails', async () => {
		mocks.setPersistence.mockRejectedValueOnce(new Error('unavailable'))
		const observer = vi.fn()
		const stop = observeMonitorAuthorization(observer)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({ status: 'error' }),
		)
		expect(mocks.onIdTokenChanged).not.toHaveBeenCalled()
		expect(mocks.checkAccess).not.toHaveBeenCalled()
		stop()
	})

	test('does not access Firestore for a signed-out session', async () => {
		const observer = vi.fn()
		const stop = observeMonitorAuthorization(observer)
		await subscribed()
		mocks.tokenObserver?.(null)
		expect(observer).toHaveBeenLastCalledWith({ status: 'signed-out' })
		expect(mocks.checkAccess).not.toHaveBeenCalled()
		stop()
	})

	test.each([undefined, false, true])(
		'allows an authenticated user after the server read succeeds regardless of the retired claim (%s)',
		async (claim) => {
			const account = {
				uid: 'monitor-user',
				getIdTokenResult: vi
					.fn()
					.mockResolvedValue({ claims: { monitor_reader: claim } }),
			} as unknown as User
			const observer = vi.fn()
			const stop = observeMonitorAuthorization(observer)
			await subscribed()
			mocks.tokenObserver?.(account)
			await vi.waitFor(() =>
				expect(observer).toHaveBeenLastCalledWith({
					status: 'authorized',
					user: account,
				}),
			)
			expect(mocks.checkAccess).toHaveBeenCalledOnce()
			expect(account.getIdTokenResult).not.toHaveBeenCalled()
			stop()
		},
	)

	test.each([
		'firestore-denied',
		'firestore-config-missing',
		'firestore-error',
	])(
		'keeps a signed-in user closed and preserves the %s diagnostic',
		async (issue) => {
			mocks.checkAccess.mockResolvedValue(issue)
			const observer = vi.fn()
			const stop = observeMonitorAuthorization(observer)
			await subscribed()
			mocks.tokenObserver?.(user())
			await vi.waitFor(() =>
				expect(observer).toHaveBeenLastCalledWith({ status: 'denied', issue }),
			)
			expect(observer).not.toHaveBeenCalledWith(
				expect.objectContaining({ status: 'authorized' }),
			)
			stop()
		},
	)

	test('keeps the same user mounted during token refresh but closes when the allowlist no longer permits reads', async () => {
		const observer = vi.fn()
		const account = user()
		const stop = observeMonitorAuthorization(observer)
		await subscribed()
		mocks.tokenObserver?.(account)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({
				status: 'authorized',
				user: account,
			}),
		)
		observer.mockClear()
		mocks.tokenObserver?.(account)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({
				status: 'authorized',
				user: account,
			}),
		)
		expect(observer).not.toHaveBeenCalledWith({ status: 'checking' })
		mocks.checkAccess.mockResolvedValueOnce('firestore-denied')
		mocks.tokenObserver?.(account)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({
				status: 'denied',
				issue: 'firestore-denied',
			}),
		)
		stop()
	})

	test('closes old data before checking a different signed-in account', async () => {
		const observer = vi.fn()
		const first = user('first')
		const stop = observeMonitorAuthorization(observer)
		await subscribed()
		mocks.tokenObserver?.(first)
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({
				status: 'authorized',
				user: first,
			}),
		)
		mocks.checkAccess.mockResolvedValueOnce('firestore-denied')
		observer.mockClear()
		mocks.tokenObserver?.(user('second'))
		expect(observer).toHaveBeenLastCalledWith({ status: 'checking' })
		await vi.waitFor(() =>
			expect(observer).toHaveBeenLastCalledWith({
				status: 'denied',
				issue: 'firestore-denied',
			}),
		)
		stop()
	})

	test.each(['sign-out', 'listener-error', 'unmount'])(
		'ignores pending authorization after %s',
		async (event) => {
			let finish: (access: string) => void = () => undefined
			mocks.checkAccess.mockReturnValue(
				new Promise((resolve) => {
					finish = resolve
				}),
			)
			const observer = vi.fn()
			const account = user()
			const stop = observeMonitorAuthorization(observer)
			await subscribed()
			mocks.tokenObserver?.(account)
			if (event === 'sign-out') mocks.tokenObserver?.(null)
			else if (event === 'listener-error') mocks.errorObserver?.()
			else stop()
			observer.mockClear()
			finish('ready')
			await Promise.resolve()
			await Promise.resolve()
			expect(observer).not.toHaveBeenCalled()
			if (event !== 'unmount') stop()
			expect(mocks.unsubscribe).toHaveBeenCalledOnce()
		},
	)
})
