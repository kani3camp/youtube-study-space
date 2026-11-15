export const monitorAccessMessages = {
	'signed-out': '未ログインです。',
	'firebase-config-missing':
		'Firebaseの接続設定が不足しています。NEXT_PUBLIC_FIREBASE_PROJECT_ID と NEXT_PUBLIC_FIREBASE_API_KEY を設定して再ビルド・再起動してください。',
	'firestore-denied':
		'Firestoreの読み取りが拒否されました。ログインしたUIDの _monitor-readers/{UID} に enabled: true が設定されていること、接続先プロジェクト、Security RulesとApp Checkの設定を確認してください。',
	'firestore-config-missing':
		'Firestoreの config/constants 文書が見つかりません。接続先プロジェクトとFirestoreの初期設定を確認してください。',
	'firestore-error':
		'Firestoreへの接続を確認できませんでした。ネットワークとFirebaseの設定を確認して、再読み込みしてください。',
} as const

export type MonitorAccessIssue = keyof typeof monitorAccessMessages

export const isMonitorAccessIssue = (
	value: unknown,
): value is MonitorAccessIssue =>
	typeof value === 'string' && Object.hasOwn(monitorAccessMessages, value)

export async function reportMonitorAccessIssue(
	issue: MonitorAccessIssue,
): Promise<void> {
	try {
		await fetch('/api/monitor-auth-required', {
			method: 'POST',
			headers: { 'X-Monitor-Auth-Required': issue },
			credentials: 'omit',
		})
	} catch {
		// 通知の成否でアクセス制御や画面表示を変えない。資格情報は送らない。
	}
}
