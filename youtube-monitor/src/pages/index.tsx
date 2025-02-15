import type { GetStaticProps } from 'next'
import { serverSideTranslations } from 'next-i18next/serverSideTranslations'
import type { FC } from 'react'
import BackgroundImage from '../components/BackgroundImage'
import BgmPlayer from '../components/BgmPlayer'
import Clock from '../components/Clock'
import Seats from '../components/MainContent'
import MenuDisplay from '../components/MenuDisplay'
import Timer from '../components/Timer'
import Usage from '../components/Usage'

const Home: FC = () => (
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
		<MenuDisplay />
		<Timer />
		<Seats />
	</div>
)

export const getStaticProps: GetStaticProps = async ({ locale }) => ({
	props: {
		...(await serverSideTranslations(locale ?? 'ja', ['common'])),
	},
})

export default Home
