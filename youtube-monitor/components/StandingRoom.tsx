import React from "react";
import styles from "./StandingRoom.module.sass";
import { NoSeatRoomState } from "../types/room-state";

class StandingRoom extends React.Component<
  { no_seat_room_state: NoSeatRoomState },
  any
> {
  render () {
    let standingUserDisplayName = '名前'
    let standingWorkName = '作業内容'
    let standingSeatColor = '#0000' // 初期値は透明
    const numNoSeatUsers = this.props.no_seat_room_state?.seats.length
    if (numNoSeatUsers !== 0) {
      const randNum = Math.floor(Math.random() * numNoSeatUsers)
      standingUserDisplayName = this.props.no_seat_room_state?.seats[randNum].user_display_name
      standingWorkName = this.props.no_seat_room_state?.seats[randNum].work_name
      standingSeatColor = this.props.no_seat_room_state?.seats[randNum].color_code
    }


    if (this.props.no_seat_room_state) {
      const numStandingWorkers = this.props.no_seat_room_state.seats.length;
      return (
        <div id={styles.standingRoom}>
          {/*<h2>Standing Room</h2>*/}
          <div className={styles.preTitle}>何人でも入れる</div>
          <h2 className={styles.title}>スタンディング</h2>
          <h3 className={styles.description}>
            （<span className={styles.commandString}><ruby>!0<rt>{'　'}ゼロ</rt></ruby></span> で入室）
            {/*（Enter with <span className={styles.commandString}>!0</span>）*/}
          </h3>

          <div className={styles.standingSeat} style={{
            backgroundColor: standingSeatColor
          }}>
            <div className={styles.standingWorkName}>{standingWorkName}</div>
            <div className={styles.standingUserDisplayName}>{standingUserDisplayName}</div>
          </div>

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
