import React, { FC } from "react";
import * as styles from "./Message.styles";
import { SeatsState } from "../types/api";

type Props = {
  default_room_state: SeatsState
}

const Message: FC<Props> = (props) => {
  if (props.default_room_state) {
    const numWorkers = props.default_room_state.seats.length
    return <div css={styles.message}>現在、{numWorkers}人が作業中🔥</div>;
  } else {
    return <div css={styles.message} />;
  }
}

export default Message;
