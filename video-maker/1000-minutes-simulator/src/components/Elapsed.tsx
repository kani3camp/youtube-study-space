import { FC } from 'react'
import * as styles from '../styles/Elapsed.styles'
import * as common from '../styles/common.styles'

type Props = {
    elapsedSeconds: number
}

const Elapsed: FC<Props> = (props) => {
    const elapsedSecondsInteger = Math.floor(props.elapsedSeconds % 60)
    const elapsedMinutesInteger = Math.floor(props.elapsedSeconds / 60)

    return (
        <div css={styles.elapsed}>
            <div css={styles.innerCell}>
                <div css={common.heading}>経過時間</div>
                <div css={styles.elapsedTime}>
                    <span>{elapsedMinutesInteger}分</span>
                    <span>{elapsedSecondsInteger}秒</span>
                </div>
            </div>
        </div>
    )
}

export default Elapsed
