import React, { FC } from "react";
import * as styles from "./Message.styles";
import { SeatsState } from "../types/api";

type Props = {
  current_room_index: number
  seats_state: SeatsState
}

const Message: FC<Props> = (props) => {
  if (props.seats_state) {
    const numWorkers = props.seats_state.seats.length
    return (
    <div css={styles.message}>
      <div css={styles.roomName}>ルーム{props.current_room_index + 1} ☝</div>
      <div css={styles.numStudyingPeople}>現在、{numWorkers}人が作業中🔥</div>
    </div>
    )
  } else {
    return <div css={styles.message} />;
  }
}

export default Message;
