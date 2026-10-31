const mocks = vi.hoisted(() => ({
	initializeApp: vi.fn((options: unknown) => ({ options })),
	getApps: vi.fn(() => []),
	getApp: vi.fn(),
}))
vi.mock('firebase/app', () => mocks)

import {
	FirebaseConfigurationError,
	getFirebaseConfig,
} from './firebase-config'
import { getFirebaseApp } from './firestore'

describe('Firebase connection configuration', () => {
	beforeEach(() => {
		vi.stubEnv('NEXT_PUBLIC_FIREBASE_PROJECT_ID', 'demo-monitor-config')
		vi.stubEnv('NEXT_PUBLIC_FIREBASE_API_KEY', 'synthetic-public-api-key')
		vi.stubEnv('NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN', undefined)
		mocks.initializeApp.mockClear()
		vi.spyOn(window, 'alert').mockImplementation(() => undefined)
	})
	afterEach(() => {
		vi.unstubAllEnvs()
		vi.restoreAllMocks()
	})

	test.each([undefined, '', '   '])(
		'initializes a fresh page without an authDomain (%s) and without an authentication alert',
		(authDomain) => {
			vi.stubEnv('NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN', authDomain)
			getFirebaseApp()
			expect(mocks.initializeApp).toHaveBeenCalledExactlyOnceWith({
				projectId: 'demo-monitor-config',
				apiKey: 'synthetic-public-api-key',
			})
			expect(window.alert).not.toHaveBeenCalled()
		},
	)

	test('preserves an explicitly configured Auth domain', () => {
		vi.stubEnv('NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN', 'auth.example.test')
		expect(getFirebaseConfig()).toEqual({
			projectId: 'demo-monitor-config',
			apiKey: 'synthetic-public-api-key',
			authDomain: 'auth.example.test',
		})
		expect(window.alert).not.toHaveBeenCalled()
	})

	test.each([
		['NEXT_PUBLIC_FIREBASE_PROJECT_ID', undefined],
		['NEXT_PUBLIC_FIREBASE_PROJECT_ID', ''],
		['NEXT_PUBLIC_FIREBASE_PROJECT_ID', '   '],
		['NEXT_PUBLIC_FIREBASE_API_KEY', undefined],
		['NEXT_PUBLIC_FIREBASE_API_KEY', ''],
		['NEXT_PUBLIC_FIREBASE_API_KEY', '   '],
	] as const)(
		'stops initialization for missing required %s (%s) without an alert',
		(name, value) => {
			vi.stubEnv(name, value)
			expect(getFirebaseApp).toThrow(FirebaseConfigurationError)
			expect(getFirebaseApp).toThrow(name)
			expect(mocks.initializeApp).not.toHaveBeenCalled()
			expect(window.alert).not.toHaveBeenCalled()
		},
	)
})
