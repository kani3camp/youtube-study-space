import { FC } from 'react'
import * as styles from '../styles/Message.styles'
import { Seat } from '../types/api'

type Props = {
    currentPageIndex: number
    currentRoomsLength: number
    seats: Seat[]
}

const Message: FC<Props> = (props) => {
    if (props.seats) {
        const numWorkers = props.seats.length
        return (
            <div css={styles.message}>
                <div css={styles.roomName}>
                    ページ{props.currentPageIndex + 1} /{' '}
                    {props.currentRoomsLength}
                </div>
                <div css={styles.numStudyingPeople}>
                    {numWorkers}人が作業中☘
                </div>
            </div>
        )
    } else {
        return <div css={styles.message} />
    }
}

export default Message
