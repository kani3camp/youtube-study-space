import { FC } from 'react'
import { MdQueueMusic } from 'react-icons/md'
import * as styles from '../styles/BGMPlayer.styles'
import * as common from '../styles/common.styles'

const BGMPlayer: FC = () => {
    return (
        <div css={styles.bgmPlayer}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <MdQueueMusic
                        size={common.IconSize}
                        css={styles.icon}
                    ></MdQueueMusic>
                    <span>BGM</span>
                </div>
                <div>title♬</div>
                <div>artist</div>
            </div>
        </div>
    )
}

export default BGMPlayer
