import { FC } from 'react'
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
    const [remainingSeconds, percentage, isStudying] = calcPomodoroRemaining(
        props.elapsedSeconds
    )

    const numberOfPomodoroRounds = calcNumberOfPomodoroRounds(
        props.elapsedSeconds
    )

    const state = isStudying ? (
        <>
            <AiFillFire
                size={styles.stateIconSize}
                css={styles.studyIcon}
            ></AiFillFire>
            集中
        </>
    ) : (
        <>
            <MdFreeBreakfast
                size={styles.stateIconSize}
                css={styles.breakIcon}
            ></MdFreeBreakfast>
            休憩
        </>
    )
    return (
        <div css={styles.timer}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <RiTimerFill
                        size={common.IconSize}
                        css={styles.icon}
                    ></RiTimerFill>
                    <span>ポモドーロタイマー</span>
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
                            {numberOfPomodoroRounds}周目
                        </div>
                        <div css={styles.isStudying}>{state}</div>
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
                    次は
                    {isStudying ? (
                        <span> 5分 休憩</span>
                    ) : (
                        <span> 25分 作業</span>
                    )}
                </div>
            </div>
        </div>
    )
}

export default Timer
