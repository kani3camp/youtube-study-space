import React, { FC } from "react";
import * as styles from "./StandingRoom.styles";

const StandingRoom: FC = () => {
  return (
    <div css={styles.standingRoom}>
      <h3 css={styles.description}>
        座席の見方
      </h3>

      <div css={styles.seat} >
        <div css={styles.seatId}>座席番号</div>
        <div css={styles.workName}>作業内容</div>
        <div css={styles.userDisplayName}>名前</div>
      </div>

      <div>
        <div>
          <span>入室する：</span><span css={styles.commandString}>!in</span>

        </div>
        <div>
          <span>退室する：</span><span css={styles.commandString}>!out</span>
        </div>
      </div>

    </div>
  );
}

export default StandingRoom;
