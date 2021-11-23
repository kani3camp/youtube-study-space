import React from "react";
import styles from "./StandingRoom.module.sass";
import { NoSeatRoomState } from "../types/room-state";

const StandingRoom = () => {
  return (
    <div id={styles.standingRoom}>
      <h3 className={styles.description}>
        座席の見方
      </h3>

      <div className={styles.seat} >
        <div className={styles.seatId}>座席番号</div>
        <div className={styles.workName}>作業内容</div>
        <div className={styles.userDisplayName}>名前</div>
      </div>

      <div>
        <div>
          <span>入室する：</span><span className={styles.commandString}>!in</span>

        </div>
        <div>
          <span>退室する：</span><span className={styles.commandString}>!out</span>
        </div>
      </div>

    </div>
  );
}

export default StandingRoom;
