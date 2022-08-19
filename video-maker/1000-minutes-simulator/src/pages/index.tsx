import { css } from '@emotion/react'
import type { NextPage } from 'next'
import { useEffect, useRef, useState } from 'react'
import BGMPlayer from '../components/BGMPlayer'
import CurrentColor from '../components/CurrentColor'
import Elapsed from '../components/Elapsed'
import Gauge from '../components/Gauge'
import Timer from '../components/Timer'
import Tips from '../components/Tips'
import { hoursToRank } from '../lib/ranks'
import * as styles from '../styles/index.styles'

const TimeUpdateIntervalMilliSec = (1 / 30) * 1000
export const OffsetSec = 7 // 画面のロード時間を考慮して、開始時間をずらす

const Home: NextPage = () => {
    const [startTime, setStartTime] = useState(0)
    const [elapsedSeconds, setElapsedSeconds] = useState<number>(0)
    const [elapsedMinutes, setElapsedMinutes] = useState<number>(0)
    const [imagePath, setImagePath] = useState<string>('')

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
        const seconds = (nowMilliSecs - startTime) / 1000 - OffsetSec
        setElapsedSeconds(seconds)
        setElapsedMinutes(Math.floor(seconds / 60))
        setImagePath(hoursToRank(elapsedMinutes).Image) // 分を時間として扱うのに注意
    })

    return (
        <div
            css={css`
                ${styles.indexStyle};
                background-image: url(${imagePath});
            `}
        >
            <BGMPlayer elapsedMinutes={elapsedMinutes}></BGMPlayer>
            <Tips elapsedSeconds={elapsedSeconds} />
            <Gauge elapsedMinutes={elapsedMinutes} />
            <CurrentColor elapsedMinutes={elapsedMinutes} />
            <Timer elapsedSeconds={elapsedSeconds} />
            <Elapsed elapsedSeconds={elapsedSeconds} />
        </div>
    )
}

export default Home
