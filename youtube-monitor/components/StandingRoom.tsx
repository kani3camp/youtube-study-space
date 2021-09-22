import React from "react";
import styles from "./StandingRoom.module.sass";
import { NoSeatRoomState } from "../types/room-state";

class StandingRoom extends React.Component<
  { no_seat_room_state: NoSeatRoomState },
  any
> {
  render () {
    if (this.props.no_seat_room_state) {
      const numStandingWorkers = this.props.no_seat_room_state.seats.length;
      return (
        <div id={styles.standingRoom}>
          {/*<h2>Standing Room</h2>*/}
          <div className={styles.preTitle}>何人でも入れる</div>
          <h2 className={styles.title}>スタンディング</h2>
          <h3>
            （<span className={styles.commandString}><ruby>!0<rt>{'　'}ゼロ</rt></ruby></span> で入室）
            {/*（Enter with <span className={styles.commandString}>!0</span>）*/}
          </h3>
          {/*<h2>{numStandingWorkers} People</h2>*/}
          <div id={styles.numStandingWorkers}>{numStandingWorkers}人</div>
        </div>
      );
    } else {
      return <div id={styles.standingRoom} />;
    }
  }
}

export default StandingRoom;
