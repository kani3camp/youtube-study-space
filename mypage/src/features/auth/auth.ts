import {
	GoogleAuthProvider,
	onAuthStateChanged,
	signInWithPopup,
	signOut,
	type User,
} from 'firebase/auth'

import { firebaseAuth } from '../../lib/firebase'

const youtubeReadonlyScope = 'https://www.googleapis.com/auth/youtube.readonly'

export type GoogleYouTubeLoginResult = {
	user: User
	idToken: string
	youtubeAccessToken: string
}

export async function signInWithGoogleAndYouTube(): Promise<GoogleYouTubeLoginResult> {
	const provider = new GoogleAuthProvider()
	provider.addScope(youtubeReadonlyScope)

	const result = await signInWithPopup(firebaseAuth, provider)
	const credential = GoogleAuthProvider.credentialFromResult(result)

	if (!credential?.accessToken) {
		throw new Error('YouTube access token is not available')
	}

	const idToken = await result.user.getIdToken()

	return {
		user: result.user,
		idToken,
		youtubeAccessToken: credential.accessToken,
	}
}

export async function signOutFirebase(): Promise<void> {
	await signOut(firebaseAuth)
}

export function waitForCurrentUser(): Promise<User | null> {
	if (firebaseAuth.currentUser) {
		return Promise.resolve(firebaseAuth.currentUser)
	}

	return new Promise((resolve) => {
		const unsubscribe = onAuthStateChanged(firebaseAuth, (user) => {
			unsubscribe()
			resolve(user)
		})
	})
}
