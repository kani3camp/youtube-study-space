import React, { FC } from 'react'
import * as styles from '../styles/Message.styles'
import { SeatsState } from '../types/api'

type Props = {
  current_room_index: number
  current_rooms_length: number
  seats_state: SeatsState
}

const Message: FC<Props> = (props) => {
  if (props.seats_state) {
    const numWorkers = props.seats_state.seats.length
    return (
      <div css={styles.message}>
        <div css={styles.roomName}>
          ルーム{props.current_room_index + 1} / {props.current_rooms_length} ☝
        </div>
        <div css={styles.numStudyingPeople}>{numWorkers}人が作業中🐟</div>
      </div>
    )
  } else {
    return <div css={styles.message} />
  }
}

export default Message
