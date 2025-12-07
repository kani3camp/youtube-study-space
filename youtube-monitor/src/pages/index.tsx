import { initializeApp } from 'firebase/app'
import {
	collection,
	getFirestore,
	onSnapshot,
	orderBy,
	query,
} from 'firebase/firestore'
import type { GetStaticProps } from 'next'
import { serverSideTranslations } from 'next-i18next/serverSideTranslations'
import { type FC, useEffect, useState } from 'react'
import BackgroundImage from '../components/BackgroundImage'
import BgmPlayer from '../components/BgmPlayer'
import Clock from '../components/Clock'
import Seats from '../components/MainContent'
import MenuDisplay from '../components/MenuDisplay'
import Timer from '../components/Timer'
import Usage from '../components/Usage'
import { firestoreMenuConverter, getFirebaseConfig } from '../lib/firestore'
import type { Menu } from '../types/api'

const Home: FC = () => {
	const [menuItems, setMenuItems] = useState<Menu[]>([])

	useEffect(() => {
		const app = initializeApp(getFirebaseConfig())
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
			<Seats menuItems={menuItems} />
		</div>
	)
}

export const getStaticProps: GetStaticProps = async ({ locale }) => ({
	props: {
		...(await serverSideTranslations(locale ?? 'ja', ['common'])),
	},
})

export default Home
