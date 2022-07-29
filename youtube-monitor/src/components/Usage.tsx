import { FC } from 'react'
import * as styles from '../styles/Usage.styles'

const StandingRoom: FC = () => (
    <div css={styles.background}>
        <div css={styles.usage}>
            <h3 css={styles.description}>座席の見方</h3>

            <div css={styles.seat}>
                <div css={styles.seatId}>座席 No.</div>
                <div css={styles.workName}>作業内容</div>
                <div css={styles.userDisplayName}>名前</div>
            </div>

            <div>
                <div>
                    <span>入室する：</span>
                    <span css={styles.commandString}>!in</span>
                </div>
                <div>
                    <span>退室する：</span>
                    <span css={styles.commandString}>!out</span>
                </div>
            </div>
        </div>
    </div>
)

export default StandingRoom
