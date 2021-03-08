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

class DefaultRoom extends React.Component<
  { layout: RoomLayout; roomState: DefaultRoomState },
  any
> {
  render() {
    return (
      <div id={styles.defaultRoom}>
        <DefaultRoomLayout
          layout={this.props.layout}
          roomState={this.props.roomState}
        />
      </div>
    );
  }
}

export default DefaultRoom;
