import { FC } from 'react'
import * as styles from '../styles/BGMPlayer.styles'
import * as common from '../styles/common.styles'

const BGMPlayer: FC = () => {
    return (
        <div css={styles.bgmPlayer}>
            <div css={styles.innerCell}>
                <div css={common.heading}>BGM</div>
                <div>title♬</div>
                <div>artist</div>
            </div>
        </div>
    )
}

export default BGMPlayer
