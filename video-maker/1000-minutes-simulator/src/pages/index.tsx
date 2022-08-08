import type { NextPage } from 'next'
import { useEffect, useRef, useState } from 'react'
import BGMPlayer from '../components/BGMPlayer'
import CurrentColor from '../components/CurrentColor'
import Elapsed from '../components/Elapsed'
import Gauge from '../components/Gauge'
import Timer from '../components/Timer'
import Tips from '../components/Tips'
import * as styles from '../styles/index.styles'

const TimeUpdateIntervalMilliSec = (1 / 30) * 1000

const Home: NextPage = () => {
    const [startTime, setStartTime] = useState(0)
    const [elapsedSeconds, setElapsedSeconds] = useState<number>(0)

    useEffect(() => {
        setStartTime(Date.now())
    }, [])

    const useInterval = (callback: () => void) => {
        const callbackRef = useRef<() => void>(callback)
        useEffect(() => {
            callbackRef.current = callback
        }, [callback])

        useEffect(() => {
            const tick = () => {
                callbackRef.current()
            }
            const id = setInterval(tick, TimeUpdateIntervalMilliSec)
            return () => {
                clearInterval(id)
            }
        }, [])
    }

    useInterval(() => {
        const nowMilliSecs = Date.now()
        setElapsedSeconds((nowMilliSecs - startTime) / 1000)
    })

    return (
        <div css={styles.indexStyle}>
            <BGMPlayer></BGMPlayer>
            <Tips />
            <Gauge elapsedMinutes={Math.floor(elapsedSeconds / 60)} />
            <CurrentColor elapsedMinutes={Math.floor(elapsedSeconds / 60)} />
            <Timer elapsedSeconds={elapsedSeconds} />
            <Elapsed elapsedSeconds={elapsedSeconds} />
        </div>
    )
}

export default Home
