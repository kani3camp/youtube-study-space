import type { FirebaseOptions } from 'firebase/app'

export class FirebaseConfigurationError extends Error {
	constructor(setting: string) {
		super(`Firebaseの接続設定が不足しています: ${setting}`)
		this.name = 'FirebaseConfigurationError'
	}
}

export function getFirebaseConfig(): FirebaseOptions {
	const projectId = process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID
	const apiKey = process.env.NEXT_PUBLIC_FIREBASE_API_KEY
	const authDomain = process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN
	if (!projectId?.trim()) {
		throw new FirebaseConfigurationError('NEXT_PUBLIC_FIREBASE_PROJECT_ID')
	}
	if (!apiKey?.trim()) {
		throw new FirebaseConfigurationError('NEXT_PUBLIC_FIREBASE_API_KEY')
	}

	return {
		projectId,
		apiKey,
		// メール・パスワード認証では不要。OAuth用ドメインを推測で補完しない。
		...(authDomain?.trim() ? { authDomain } : {}),
	}
}
