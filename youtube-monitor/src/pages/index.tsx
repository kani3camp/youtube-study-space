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
        <Seats />
        <StandingRoom />
        <Timer />
    </div>
)
export default Home
