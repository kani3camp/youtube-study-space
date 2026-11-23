import type { Seat, WorkNameTrend } from '../types/api'
import type { SystemConstants } from './firestore'
import { MonitorConfigMissingError } from './monitor-access-issues'

type ConstantsSnapshotLike = {
	exists: () => boolean
	data: () => SystemConstants | undefined
}

type WorkNameTrendSnapshotLike = {
	docs: { data: () => WorkNameTrend }[]
}

/**
 * `config/constants` のスナップショットを検証して通知する。
 * ドキュメントが存在しない場合は、不完全な値で最大席数を判断しないよう、
 * 'firestore-config-missing' に分類されるエラーを onReadError に通知する。原因は console にも出す。
 * data() は 1 回だけ読む。
 */
export const handleConstantsSnapshot = (
	snapshot: ConstantsSnapshotLike,
	onConstants: (constants: SystemConstants) => void,
	onReadError: (error: unknown) => void,
): void => {
	const constants = snapshot.exists() ? snapshot.data() : undefined
	if (constants === undefined) {
		const error = new MonitorConfigMissingError(
			"Firestore document 'config/constants' does not exist.",
		)
		console.error(error.message)
		onReadError(error)
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
		const error = new Error(
			`Found ${snapshot.docs.length} work name trend documents in Firestore, but only one is expected. Please ensure that only one document exists in the 'work-name-trend' collection.`,
		)
		console.error(error.message)
		onReadError(error)
		return
	}
	onWorkNameTrend(snapshot.docs[0].data())
}

type SeatsSnapshotLike = {
	docs: { data: () => Seat }[]
}

/**
 * `seats` / `member-seats` コレクションのスナップショットを座席配列に変換して通知する。
 * onSnapshot の next コールバック内の例外はエラーコールバックに転送されないため、
 * 非対応スキーマ等で data() が例外を投げた場合は onReadError に通知する。
 */
export const handleSeatsSnapshot = (
	snapshot: SeatsSnapshotLike,
	onSeats: (seats: Seat[]) => void,
	onReadError: (error: unknown) => void,
): void => {
	let seats: Seat[]
	try {
		seats = snapshot.docs.map((doc) => doc.data())
	} catch (error) {
		console.error('failed to read seats snapshot', error)
		onReadError(error)
		return
	}
	onSeats(seats)
}
