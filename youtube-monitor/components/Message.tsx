import React from "react";
import * as styles from "./Message.styles";
import { DefaultRoomState, NoSeatRoomState } from "../types/room-state";

class Message extends React.Component<
  { default_room_state: DefaultRoomState },
  any
> {
  render () {
    if (this.props.default_room_state) {
      const numWorkers = this.props.default_room_state.seats.length
      // return (
      //   <div id={styles.message}>Currently {numWorkers} people working! 🔥</div>
      // );
      return <div css={styles.message}>現在、{numWorkers}人が作業中🔥</div>;
    } else {
      return <div css={styles.message} />;
    }
  }
}

export default Message;
