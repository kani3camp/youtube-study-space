import {
	collection,
	doc,
	getFirestore,
	onSnapshot,
	orderBy,
	query,
	Timestamp,
} from 'firebase/firestore'
import { useEffect, useMemo, useState } from 'react'
import {
	firestoreConstantsConverter,
	firestoreMenuConverter,
	firestoreSeatConverter,
	firestoreWorkNameTrendConverter,
	getFirebaseApp,
	type SystemConstants,
} from '../lib/firestore'
import type { Menu, Seat, WorkNameTrend } from '../types/api'

const DATA_SOURCES = [
	'generalSeats',
	'memberSeats',
	'constants',
	'workNameTrend',
	'menu',
] as const

type DataSource = (typeof DATA_SOURCES)[number]
type LoadedSources = Record<DataSource, boolean>

const initialLoadedSources = (): LoadedSources => ({
	generalSeats: false,
	memberSeats: false,
	constants: false,
	workNameTrend: false,
	menu: false,
})

const emptyWorkNameTrend = (): WorkNameTrend => ({
	ranking: [],
	ranked_at: Timestamp.fromMillis(0),
})

export type MonitorData = {
	generalSeats: Seat[]
	memberSeats: Seat[]
	systemConstants?: SystemConstants
	workNameTrend: WorkNameTrend
	menuItems: Menu[]
	menuImageMap: Map<string, string>
	loading: boolean
	error?: Error
}

const useMonitorData = (): MonitorData => {
	const [generalSeats, setGeneralSeats] = useState<Seat[]>([])
	const [memberSeats, setMemberSeats] = useState<Seat[]>([])
	const [systemConstants, setSystemConstants] = useState<SystemConstants>()
	const [workNameTrend, setWorkNameTrend] =
		useState<WorkNameTrend>(emptyWorkNameTrend)
	const [menuItems, setMenuItems] = useState<Menu[]>([])
	const [loadedSources, setLoadedSources] =
		useState<LoadedSources>(initialLoadedSources)
	const [error, setError] = useState<Error>()

	useEffect(() => {
		const db = getFirestore(getFirebaseApp())

		const markLoaded = (source: DataSource) => {
			setLoadedSources((previous) => ({ ...previous, [source]: true }))
		}
		const handleError = (source: DataSource, cause: Error) => {
			console.error(`Firestore listener failed for ${source}:`, cause)
			setError(cause)
			markLoaded(source)
		}

		const unsubscribeGeneralSeats = onSnapshot(
			query(collection(db, 'seats')).withConverter(firestoreSeatConverter),
			(snapshot) => {
				setGeneralSeats(
					snapshot.docs.map((documentSnapshot) => documentSnapshot.data()),
				)
				markLoaded('generalSeats')
			},
			(cause) => handleError('generalSeats', cause),
		)
		const unsubscribeMemberSeats = onSnapshot(
			query(collection(db, 'member-seats')).withConverter(
				firestoreSeatConverter,
			),
			(snapshot) => {
				setMemberSeats(
					snapshot.docs.map((documentSnapshot) => documentSnapshot.data()),
				)
				markLoaded('memberSeats')
			},
			(cause) => handleError('memberSeats', cause),
		)
		const unsubscribeConstants = onSnapshot(
			doc(db, 'config', 'constants').withConverter(firestoreConstantsConverter),
			(documentSnapshot) => {
				const data = documentSnapshot.data()
				if (data === undefined) {
					handleError(
						'constants',
						new Error("Firestore document 'config/constants' was not found."),
					)
					return
				}
				setSystemConstants(data)
				markLoaded('constants')
			},
			(cause) => handleError('constants', cause),
		)
		const unsubscribeWorkNameTrend = onSnapshot(
			query(collection(db, 'work-name-trend')).withConverter(
				firestoreWorkNameTrendConverter,
			),
			(snapshot) => {
				if (snapshot.docs.length > 1) {
					handleError(
						'workNameTrend',
						new Error(
							`Found ${snapshot.docs.length} work name trend documents in Firestore, but only one is expected.`,
						),
					)
					return
				}
				setWorkNameTrend(snapshot.docs[0]?.data() ?? emptyWorkNameTrend())
				markLoaded('workNameTrend')
			},
			(cause) => handleError('workNameTrend', cause),
		)
		const unsubscribeMenu = onSnapshot(
			query(collection(db, 'menu'), orderBy('code', 'asc')).withConverter(
				firestoreMenuConverter,
			),
			(snapshot) => {
				setMenuItems(
					snapshot.docs.map((documentSnapshot) => documentSnapshot.data()),
				)
				markLoaded('menu')
			},
			(cause) => handleError('menu', cause),
		)

		return () => {
			unsubscribeGeneralSeats()
			unsubscribeMemberSeats()
			unsubscribeConstants()
			unsubscribeWorkNameTrend()
			unsubscribeMenu()
		}
	}, [])

	const menuImageMap = useMemo(() => {
		const map = new Map<string, string>()
		for (const item of menuItems) {
			map.set(item.code, item.image || '/images/menu_default.svg')
		}
		return map
	}, [menuItems])

	const loading = DATA_SOURCES.some((source) => !loadedSources[source])

	return {
		generalSeats,
		memberSeats,
		systemConstants,
		workNameTrend,
		menuItems,
		menuImageMap,
		loading,
		error,
	}
}

export default useMonitorData
