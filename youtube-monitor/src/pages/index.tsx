import {
	collection,
	getFirestore,
	onSnapshot,
	orderBy,
	query,
} from 'firebase/firestore'
import type { GetStaticProps } from 'next'
import { serverSideTranslations } from 'next-i18next/pages/serverSideTranslations'
import { type FC, useCallback, useEffect, useState } from 'react'
import AmbientFrame from '../components/AmbientFrame'
import BackgroundImage from '../components/BackgroundImage'
import BgmPlayer from '../components/BgmPlayer'
import Clock from '../components/Clock'
import ColorBar from '../components/ColorBar'
import Seats from '../components/MainContent'
import MenuDisplay from '../components/MenuDisplay'
import MonitorAccessStatus from '../components/MonitorAccessStatus'
import MonitorAuthGate from '../components/MonitorAuthGate'
import Timer from '../components/Timer'
import Usage from '../components/Usage'
import { useTimeTheme } from '../hooks/use-time-theme'
import { firestoreMenuConverter, getFirebaseApp } from '../lib/firestore'
import { reportMonitorAccessIssue } from '../lib/monitor-access-issues'
import {
	classifyMonitorFirestoreError,
	type MonitorFirestoreIssue,
} from '../lib/monitor-firestore-access'
import { themedRoot } from '../styles/AmbientFrame.styles'
import type { Menu } from '../types/api'

export const Monitor: FC = () => {
	const [menuItems, setMenuItems] = useState<Menu[]>([])
	const [readFailure, setReadFailure] = useState<MonitorFirestoreIssue | null>(
		null,
	)
	const { timeTheme, textTone, contrastBridge } = useTimeTheme()
	const handleReadError = useCallback((error: unknown) => {
		setReadFailure((current) => current ?? classifyMonitorFirestoreError(error))
	}, [])
	useEffect(() => {
		if (readFailure) {
			void reportMonitorAccessIssue(readFailure)
		}
	}, [readFailure])

	useEffect(() => {
		if (readFailure) {
			return undefined
		}

		const app = getFirebaseApp()
		const db = getFirestore(app)

		const menuQuery = query(
			collection(db, 'menu'),
			orderBy('code', 'asc'),
		).withConverter(firestoreMenuConverter)

		const unsubscribe = onSnapshot(
			menuQuery,
			(querySnapshot) => {
				const items: Menu[] = []
				for (const doc of querySnapshot.docs) {
					items.push(doc.data())
				}
				setMenuItems(items)
			},
			handleReadError,
		)

		return () => {
			unsubscribe()
		}
	}, [handleReadError, readFailure])

	if (readFailure) {
		return (
			<MonitorAccessStatus
				title="配信画面を表示できません"
				detail="ライブチャット、または緊急連絡フォームからご連絡ください。"
			/>
		)
	}

	return (
		<div
			css={themedRoot}
			data-time-theme={timeTheme}
			data-text-tone={textTone}
			data-contrast-bridge={contrastBridge ? 'true' : undefined}
			style={{
				height: 1080,
				width: 1920,
				margin: 0,
				position: 'relative',
			}}
		>
			<BackgroundImage />
			<BgmPlayer />
			<Clock />
			<Usage />
			<MenuDisplay menuItems={menuItems} />
			<Timer />
			<ColorBar />
			<Seats menuItems={menuItems} onReadError={handleReadError} />
			<AmbientFrame />
		</div>
	)
}

const Home: FC = () => (
	<MonitorAuthGate>
		<Monitor />
	</MonitorAuthGate>
)

export const getStaticProps: GetStaticProps = async ({ locale }) => ({
	props: {
		...(await serverSideTranslations(locale ?? 'ja', ['common'])),
	},
})

export default Home
