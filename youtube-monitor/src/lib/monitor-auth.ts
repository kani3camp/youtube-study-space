import {
	browserLocalPersistence,
	getAuth,
	onIdTokenChanged,
	setPersistence,
	signInWithEmailAndPassword,
	signOut,
	type User,
} from 'firebase/auth'
import { getFirebaseApp } from './firestore'
import {
	checkMonitorFirestoreAccess,
	type MonitorFirestoreIssue,
} from './monitor-firestore-access'

export type MonitorAuthorizationState =
	| { status: 'checking' }
	| { status: 'signed-out' }
	| { status: 'denied'; issue: MonitorFirestoreIssue }
	| { status: 'authorized'; user: User }
	| { status: 'error' }

type AuthorizationObserver = (state: MonitorAuthorizationState) => void

const getMonitorAuth = () => getAuth(getFirebaseApp())

export const observeMonitorAuthorization = (
	observer: AuthorizationObserver,
): (() => void) => {
	let stopped = false
	let tokenRevision = 0
	let authorizedUid: string | null = null
	let unsubscribe: () => void = () => undefined

	void Promise.resolve()
		.then(async () => {
			if (stopped) {
				return
			}
			const auth = getMonitorAuth()
			await setPersistence(auth, browserLocalPersistence)
			if (stopped) {
				return
			}

			unsubscribe = onIdTokenChanged(
				auth,
				(user) => {
					if (stopped) {
						return
					}
					const revision = ++tokenRevision
					if (user === null) {
						authorizedUid = null
						observer({ status: 'signed-out' })
						return
					}

					if (authorizedUid !== user.uid) {
						observer({ status: 'checking' })
					}
					void checkMonitorFirestoreAccess()
						.then((access) => {
							if (!stopped && revision === tokenRevision) {
								authorizedUid = access === 'ready' ? user.uid : null
								observer(
									access === 'ready'
										? { status: 'authorized', user }
										: { status: 'denied', issue: access },
								)
							}
						})
						.catch(() => {
							if (!stopped && revision === tokenRevision) {
								authorizedUid = null
								observer({ status: 'error' })
							}
						})
				},
				() => {
					if (!stopped) {
						tokenRevision++
						authorizedUid = null
						observer({ status: 'error' })
					}
				},
			)
		})
		.catch(() => {
			if (!stopped) {
				observer({ status: 'error' })
			}
		})

	return () => {
		stopped = true
		tokenRevision++
		unsubscribe()
	}
}

export const signInMonitor = async (
	email: string,
	password: string,
): Promise<void> => {
	const auth = getMonitorAuth()
	await setPersistence(auth, browserLocalPersistence)
	await signInWithEmailAndPassword(auth, email, password)
}

export const signOutMonitor = async (): Promise<void> => {
	await signOut(getMonitorAuth())
}
