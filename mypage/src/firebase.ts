import { initializeApp } from 'firebase/app'
import {
	getToken,
	initializeAppCheck,
	ReCaptchaEnterpriseProvider,
} from 'firebase/app-check'
import {
	browserLocalPersistence,
	initializeAuth,
	onAuthStateChanged,
	signInWithCustomToken,
	signOut,
} from 'firebase/auth'
import { RequestError } from './features/mypage/memory'
import { BrowserRuntime, type BrowserSession } from './runtime'

// Public Firebase configuration only. No admin credential, OAuth client secret,
// debug token, user metadata or MyPage response belongs in Vite configuration.
export function initializeRuntime() {
	const env = import.meta.env
	const policy = {
		privacy: env.VITE_PRIVACY_POLICY_VERSION ?? '',
		terms: env.VITE_TERMS_VERSION ?? '',
	}
	const config = {
		apiKey: env.VITE_FIREBASE_API_KEY,
		authDomain: env.VITE_FIREBASE_AUTH_DOMAIN,
		projectId: env.VITE_FIREBASE_PROJECT_ID,
		appId: env.VITE_FIREBASE_APP_ID,
	}
	if (
		!Object.values(config).every(Boolean) ||
		!env.VITE_APP_CHECK_SITE_KEY ||
		!policy.privacy ||
		!policy.terms
	)
		return new BrowserRuntime(null, policy, fetch, 'LOGIN_UNAVAILABLE')
	try {
		const app = initializeApp(config)
		const auth = initializeAuth(app, { persistence: browserLocalPersistence })
		const appCheck = initializeAppCheck(app, {
			provider: new ReCaptchaEnterpriseProvider(env.VITE_APP_CHECK_SITE_KEY),
			isTokenAutoRefreshEnabled: true,
		})
		const session: BrowserSession = {
			currentUID: () => auth.currentUser?.uid ?? null,
			idToken: async (uid, force) => {
				if (!auth.currentUser || auth.currentUser.uid !== uid)
					throw new RequestError(401, 'AUTH_REQUIRED')
				return auth.currentUser.getIdToken(force)
			},
			appCheck: async (force) => {
				try {
					return (await getToken(appCheck, force)).token
				} catch {
					throw new RequestError(403, 'APP_CHECK_REQUIRED')
				}
			},
			subscribe: (listener) =>
				onAuthStateChanged(
					auth,
					(user) => listener(user?.uid ?? null),
					() => listener(null),
				),
			recheck: async () => {
				await auth.authStateReady()
				return auth.currentUser?.uid ?? null
			},
			signIn: async (token) =>
				(await signInWithCustomToken(auth, token)).user.uid,
			signOut: () => signOut(auth),
		}
		return new BrowserRuntime(
			session,
			policy,
			fetch,
			null,
			env.VITE_PRIVACY_INTAKE_ENABLED === 'true',
		)
	} catch {
		return new BrowserRuntime(null, policy, fetch, 'SUPPORTED_BROWSER_REQUIRED')
	}
}
