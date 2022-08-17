import { FC } from 'react'
import { HiClock } from 'react-icons/hi'
import * as styles from '../styles/Elapsed.styles'
import * as common from '../styles/common.styles'

type Props = {
    elapsedSeconds: number
}

const Elapsed: FC<Props> = (props) => {
    const elapsedSecondsInteger = Math.floor(props.elapsedSeconds % 60)
    const elapsedMinutesInteger = Math.floor(props.elapsedSeconds / 60)
    const elapsedHoursInteger = Math.floor(elapsedMinutesInteger / 60)

    return (
        <div css={styles.elapsed}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <HiClock size={common.IconSize} css={styles.icon}></HiClock>
                    <span>経過時間</span>
                </div>
                <div css={styles.elapsedTime}>
                    <span>{elapsedMinutesInteger}</span>
                    <span css={styles.elapsedTimeSubscript}>分</span>

                    <span css={styles.elapsedTimeSubscript}>
                        {String(elapsedSecondsInteger).padStart(2, '0')}秒
                    </span>
                </div>
                <div css={styles.elapsedTime2}>
                    {'( '}
                    <span>{elapsedHoursInteger}</span>
                    <span css={styles.elapsedTimeSubscript}>時間</span>
                    <span>{elapsedMinutesInteger % 60}</span>
                    <span css={styles.elapsedTimeSubscript}>分</span>
                    <span>
                        {String(elapsedSecondsInteger).padStart(2, '0')}
                    </span>
                    <span css={styles.elapsedTimeSubscript}>秒</span>
                    {')'}
                </div>
            </div>
        </div>
    )
}

export default Elapsed
