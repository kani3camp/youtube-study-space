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
import { themedRoot } from '../styles/AmbientFrame.styles'
import type { Menu } from '../types/api'

export const Monitor: FC = () => {
	const [menuItems, setMenuItems] = useState<Menu[]>([])
	const [readFailed, setReadFailed] = useState(false)
	const { timeTheme, textTone, contrastBridge } = useTimeTheme()
	const handleReadError = useCallback(() => setReadFailed(true), [])

	useEffect(() => {
		if (readFailed) {
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
	}, [handleReadError, readFailed])

	if (readFailed) {
		return (
			<MonitorAccessStatus
				title="Firestore を読み込めません"
				detail="読み取り権限またはネットワークを確認してください。安全のためモニターを停止しています。"
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
