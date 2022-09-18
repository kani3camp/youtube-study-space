import { GetStaticProps } from 'next'
import { serverSideTranslations } from 'next-i18next/serverSideTranslations'
import { FC } from 'react'
import BackgroundImage from '../components/BackgroundImage'
import BgmPlayer from '../components/BgmPlayer'
import Clock from '../components/Clock'
import Seats from '../components/Seats'
import Timer from '../components/Timer'
import StandingRoom from '../components/Usage'

const Home: FC = () => (
    <div
        style={{
            height: 1080,
            width: 1920,
            margin: 0,
            position: 'relative',
        }}
    >
        <BackgroundImage></BackgroundImage>
        <BgmPlayer></BgmPlayer>
        <Clock />
        <StandingRoom />
        <Timer />
        <Seats />
    </div>
)

export const getStaticProps: GetStaticProps = async ({ locale }) => ({
    props: {
        ...(await serverSideTranslations(locale ?? 'jp', ['common'])),
    },
})

export default Home
