import React, { FC } from 'react'
import * as styles from '../styles/Message.styles'
import { Seat } from '../types/api'

type Props = {
    current_room_index: number
    current_rooms_length: number
    seats: Seat[]
}

const Message: FC<Props> = (props) => {
    if (props.seats) {
        const numWorkers = props.seats.length
        return (
            <div css={styles.message}>
                <div css={styles.roomName}>
                    ページ{props.current_room_index + 1} /{' '}
                    {props.current_rooms_length} ☝
                </div>
                <div css={styles.numStudyingPeople}>
                    {numWorkers}人が作業中🎈
                </div>
            </div>
        )
    } else {
        return <div css={styles.message} />
    }
}

export default Message
