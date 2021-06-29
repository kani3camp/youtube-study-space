import React from "react";
import styles from "./StandingRoom.module.sass";
import { NoSeatRoomState } from "../types/room-state";

class StandingRoom extends React.Component<
  { no_seat_room_state: NoSeatRoomState },
  any
> {
  render() {
    if (this.props.no_seat_room_state) {
      const numStandingWorkers = this.props.no_seat_room_state.seats.length;
      return (
        <div id={styles.standingRoom}>
          <h2>　</h2>
          {/*<h2>Standing Room</h2>*/}
          <h2>スタンディング</h2>
          <h3>
            （<span className={styles.commandString}>!0</span> で入室）
            {/*（Enter with <span className={styles.commandString}>!0</span>）*/}
          </h3>
          {/*<h2>{numStandingWorkers} People</h2>*/}
          <h2>{numStandingWorkers}人</h2>
        </div>
      );
    } else {
      return <div id={styles.standingRoom} />;
    }
  }
}

export default StandingRoom;
