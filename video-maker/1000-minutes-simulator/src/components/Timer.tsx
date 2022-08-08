import { FC } from 'react'
import {
    buildStyles,
    CircularProgressbarWithChildren,
} from 'react-circular-progressbar'
import { calcPomodoroRemaining } from '../lib/common'

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
    return (
        <div css={styles.timer}>
            <div css={styles.innerCell}>
                <div css={common.heading}>ポモドーロタイマー</div>
                <div css={styles.progressBarContainer}>
                    <CircularProgressbarWithChildren
                        value={Number(percentage)}
                        styles={buildStyles({
                            strokeLinecap: 'butt',
                            pathTransitionDuration: 0,
                            pathColor: isStudying ? 'red' : 'limegreen',
                        })}
                    >
                        <div>{isStudying ? '集中' : '休憩'}</div>
                        <div>
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
                        <span>5分 休憩</span>
                    ) : (
                        <span>25分 作業</span>
                    )}
                </div>
            </div>
        </div>
    )
}

export default Timer
