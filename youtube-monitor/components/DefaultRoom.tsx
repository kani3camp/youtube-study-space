import React from "react";
import styles from "./DefaultRoom.module.sass";
import fetcher from "../lib/fetcher";
import {
  DefaultRoomState,
  RoomsStateResponse,
  seat,
} from "../types/room-state";
import DefaultRoomLayout from "./DefaultRoomLayout";
import { RoomLayout } from "../types/room-layout";

const DefaultRoom = () => {
  return (
    <div id={styles.defaultRoom}>
      <DefaultRoomLayout />
    </div>
  );
}

export default DefaultRoom;
