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

export type MonitorAuthorizationState =
	| { status: 'checking' }
	| { status: 'signed-out' }
	| { status: 'unauthorized' }
	| { status: 'authorized'; user: User }
	| { status: 'error' }

type AuthorizationObserver = (state: MonitorAuthorizationState) => void

const getMonitorAuth = () => getAuth(getFirebaseApp())

export const hasMonitorReaderClaim = async (user: User): Promise<boolean> => {
	const token = await user.getIdTokenResult()
	return token.claims.monitor_reader === true
}

export const observeMonitorAuthorization = (
	observer: AuthorizationObserver,
): (() => void) => {
	const auth = getMonitorAuth()
	let stopped = false
	let tokenRevision = 0
	let authorizedUid: string | null = null
	let unsubscribe: () => void = () => undefined

	void setPersistence(auth, browserLocalPersistence)
		.then(() => {
			if (stopped) {
				return
			}

			unsubscribe = onIdTokenChanged(
				auth,
				(user) => {
					const revision = ++tokenRevision
					if (user === null) {
						authorizedUid = null
						observer({ status: 'signed-out' })
						return
					}

					if (authorizedUid !== user.uid) {
						observer({ status: 'checking' })
					}
					void hasMonitorReaderClaim(user)
						.then((authorized) => {
							if (!stopped && revision === tokenRevision) {
								authorizedUid = authorized ? user.uid : null
								observer(
									authorized
										? { status: 'authorized', user }
										: { status: 'unauthorized' },
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
