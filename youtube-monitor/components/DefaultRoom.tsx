import React from "react";
import * as styles from "./DefaultRoom.styles";
import DefaultRoomLayout from "./DefaultRoomLayout";
import { RoomLayout } from "../types/room-layout";

const DefaultRoom = () => {
  return (
    <div css={styles.defaultRoom}>
      <DefaultRoomLayout />
    </div>
  );
}

export default DefaultRoom;
