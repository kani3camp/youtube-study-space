import {
	collection,
	getFirestore,
	onSnapshot,
	orderBy,
	query,
} from 'firebase/firestore'
import type { GetStaticProps } from 'next'
import { serverSideTranslations } from 'next-i18next/pages/serverSideTranslations'
import { type FC, useEffect, useState } from 'react'
import AmbientFrame from '../components/AmbientFrame'
import BackgroundImage from '../components/BackgroundImage'
import BgmPlayer from '../components/BgmPlayer'
import Clock from '../components/Clock'
import ColorBar from '../components/ColorBar'
import Seats from '../components/MainContent'
import MenuDisplay from '../components/MenuDisplay'
import Timer from '../components/Timer'
import Usage from '../components/Usage'
import { useTimeTheme } from '../hooks/use-time-theme'
import { firestoreMenuConverter, getFirebaseApp } from '../lib/firestore'
import { themedRoot } from '../styles/AmbientFrame.styles'
import type { Menu } from '../types/api'

const Home: FC = () => {
	const [menuItems, setMenuItems] = useState<Menu[]>([])
	const { timeTheme, textTone, contrastBridge } = useTimeTheme()

	useEffect(() => {
		const app = getFirebaseApp()
		const db = getFirestore(app)

		const menuQuery = query(
			collection(db, 'menu'),
			orderBy('code', 'asc'),
		).withConverter(firestoreMenuConverter)

		const unsubscribe = onSnapshot(menuQuery, (querySnapshot) => {
			const items: Menu[] = []
			for (const doc of querySnapshot.docs) {
				items.push(doc.data())
			}
			setMenuItems(items)
		})

		return () => {
			unsubscribe()
		}
	}, [])

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
			<Seats menuItems={menuItems} />
			<AmbientFrame />
		</div>
	)
}

export const getStaticProps: GetStaticProps = async ({ locale }) => ({
	props: {
		...(await serverSideTranslations(locale ?? 'ja', ['common'])),
	},
})

export default Home
