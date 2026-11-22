import type { WorkNameTrend } from '../types/api'
import type { SystemConstants } from './firestore'

type ConstantsSnapshotLike = {
	exists: () => boolean
	data: () => SystemConstants | undefined
}

type WorkNameTrendSnapshotLike = {
	docs: { data: () => WorkNameTrend }[]
}

/**
 * `config/constants` のスナップショットを検証して通知する。
 * ドキュメントが存在しない場合は、不完全な値で最大席数を判断しないよう onReadError に通知する。
 * data() は 1 回だけ読む。
 */
export const handleConstantsSnapshot = (
	snapshot: ConstantsSnapshotLike,
	onConstants: (constants: SystemConstants) => void,
	onReadError: (error: unknown) => void,
): void => {
	const constants = snapshot.exists() ? snapshot.data() : undefined
	if (constants === undefined) {
		onReadError(
			new Error("Firestore document 'config/constants' does not exist."),
		)
		return
	}
	onConstants(constants)
}

/**
 * `work-name-trend` コレクションのスナップショットを検証して通知する。
 * 0 件の場合は何もしない。複数件の場合は例外を投げず onReadError に通知する。
 */
export const handleWorkNameTrendSnapshot = (
	snapshot: WorkNameTrendSnapshotLike,
	onWorkNameTrend: (workNameTrend: WorkNameTrend) => void,
	onReadError: (error: unknown) => void,
): void => {
	if (snapshot.docs.length === 0) {
		return
	}
	if (snapshot.docs.length > 1) {
		onReadError(
			new Error(
				`Found ${snapshot.docs.length} work name trend documents in Firestore, but only one is expected. Please ensure that only one document exists in the 'work-name-trend' collection.`,
			),
		)
		return
	}
	onWorkNameTrend(snapshot.docs[0].data())
}
