import React from "react";
import styles from "./Message.module.sass";
import { DefaultRoomState, NoSeatRoomState } from "../types/room-state";

class Message extends React.Component<
  { default_room_state: DefaultRoomState; no_seat_room_state: NoSeatRoomState },
  any
> {
  render() {
    if (this.props.default_room_state && this.props.no_seat_room_state) {
      const numWorkers =
        this.props.default_room_state.seats.length +
        this.props.no_seat_room_state.seats.length;
      return (
        <div id={styles.message}>Currently {numWorkers} people working! 🔥</div>
      );
      // return <div id={styles.message}>現在、{numWorkers}人が作業中🔥</div>;
    } else {
      return <div id={styles.message} />;
    }
  }
}

export default Message;
