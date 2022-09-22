import { css } from '@emotion/react'
import { useTranslation } from 'next-i18next'
import { FC, useEffect, useState } from 'react'
import {
    buildStyles,
    CircularProgressbarWithChildren,
} from 'react-circular-progressbar'
import { AiFillFire } from 'react-icons/ai'
import { MdFreeBreakfast } from 'react-icons/md'
import { RiTimerFill } from 'react-icons/ri'
import {
    calcNumberOfPomodoroRounds,
    calcPomodoroRemaining,
} from '../lib/common'

import * as styles from '../styles/Timer.styles'
import * as common from '../styles/common.styles'
import 'react-circular-progressbar/dist/styles.css'

type Props = {
    elapsedSeconds: number
}

const Timer: FC<Props> = (props) => {
    const { t } = useTranslation()

    const chime1DivId = 'chime1'
    const chime2DivId = 'chime2'

    const [isStudyingState, setIsStudyingState] = useState<boolean>(false)

    const [remainingSeconds, percentage, isStudying] = calcPomodoroRemaining(
        props.elapsedSeconds
    )
    if (isStudying !== isStudyingState) {
        setIsStudyingState(isStudying as boolean)
    }

    const numberOfPomodoroRounds = calcNumberOfPomodoroRounds(
        props.elapsedSeconds
    )

    const stateHTML = isStudying ? (
        <>
            <AiFillFire
                size={styles.stateIconSize}
                css={styles.studyIcon}
            ></AiFillFire>
            <span
                css={css`
                    vertical-align: middle;
                `}
            >
                {t('timer.focus')}
            </span>
        </>
    ) : (
        <>
            <MdFreeBreakfast
                size={styles.stateIconSize}
                css={styles.breakIcon}
            ></MdFreeBreakfast>
            <span
                css={css`
                    vertical-align: middle;
                `}
            >
                {t('timer.break')}
            </span>
        </>
    )

    useEffect(() => {
        if (isStudyingState) {
            chime1Play()
        } else {
            chime2Play()
        }
    }, [isStudyingState])

    const chime1Play = () => {
        console.log(chime1Play.name)
        const chime1 = document.getElementById(chime1DivId) as HTMLAudioElement
        chime1.volume = 0.8
        chime1.play()
    }

    const chime2Play = () => {
        console.log(chime2Play.name)
        const chime2 = document.getElementById(chime2DivId) as HTMLAudioElement
        chime2.volume = 0.8
        chime2.play()
    }

    return (
        <div css={styles.timer}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <RiTimerFill
                        size={common.IconSize}
                        css={styles.icon}
                    ></RiTimerFill>
                    <span>{t('timer.title')}</span>
                </div>
                <div css={styles.progressBarContainer}>
                    <CircularProgressbarWithChildren
                        value={Number(percentage)}
                        styles={buildStyles({
                            strokeLinecap: 'butt',
                            pathTransitionDuration: 0,
                            pathColor: isStudying ? 'orangered' : 'lime',
                        })}
                    >
                        <div css={styles.numberOfRoundsString}>
                            {t('timer.round', {
                                value: numberOfPomodoroRounds,
                            })}
                        </div>
                        <div css={styles.isStudying}>{stateHTML}</div>
                        <div css={styles.remaining}>
                            {String(
                                Math.floor(Number(remainingSeconds) / 60)
                            ).padStart(2, '0')}
                            :
                            {String(
                                Math.floor(Number(remainingSeconds) % 60)
                            ).padStart(2, '0')}
                        </div>
                    </CircularProgressbarWithChildren>
                </div>
                <div>
                    {t('timer.next')}
                    {isStudying ? (
                        <span>{` ${t('timer.next_rest', { value: 5 })}`}</span>
                    ) : (
                        <span>{` ${t('timer.next_work', { value: 5 })}`}</span>
                    )}
                </div>
            </div>

            <audio id={chime1DivId} src='/audio/chime/chime1.mp3'></audio>
            <audio id={chime2DivId} src='/audio/chime/chime2.mp3'></audio>
        </div>
    )
}

export default Timer
