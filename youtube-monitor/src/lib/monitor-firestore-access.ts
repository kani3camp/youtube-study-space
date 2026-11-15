import { doc, getDocFromServer, getFirestore } from 'firebase/firestore'
import { getFirebaseApp } from './firestore'

export type MonitorFirestoreIssue =
	| 'firestore-denied'
	| 'firestore-config-missing'
	| 'firestore-error'

export const classifyMonitorFirestoreError = (
	error: unknown,
): MonitorFirestoreIssue =>
	typeof error === 'object' &&
	error !== null &&
	'code' in error &&
	error.code === 'permission-denied'
		? 'firestore-denied'
		: 'firestore-error'

export async function checkMonitorFirestoreAccess(): Promise<
	'ready' | MonitorFirestoreIssue
> {
	try {
		// 許可リストを直接読まず、実際のRulesが許可する既存文書で確認する。
		// キャッシュだけで成功と判断せず、文書の内容も診断には使わない。
		const snapshot = await getDocFromServer(
			doc(getFirestore(getFirebaseApp()), 'config', 'constants'),
		)
		return snapshot.exists() ? 'ready' : 'firestore-config-missing'
	} catch (error) {
		return classifyMonitorFirestoreError(error)
	}
}
