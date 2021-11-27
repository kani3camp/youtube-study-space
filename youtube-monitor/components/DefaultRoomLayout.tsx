import React, { useState, useEffect, useReducer, FC } from "react";
import * as styles from "./DefaultRoomLayout.styles";
import { RoomLayout } from "../types/room-layout";
import { Seat, SeatsState } from "../types/room-state";

type Props = {
  roomLayout: RoomLayout;
  seats: Seat[];
  firstSeatId: number;
}

const DefaultRoomLayout: FC<Props> = (props) => {
  return (
    <div>
        <div
          css={styles.roomLayout}
        >
          {/* {seatList} */}
          {
            props.seats.map((eachSeat) => {
              const workName = eachSeat.work_name
              const displayName = eachSeat.user_display_name
              const seatColor = props.seats.find(s => s.seat_id === eachSeat.seat_id)?.color_code;

              return (
                <div
                  key={eachSeat.seat_id}
                  css={styles.seat}
                  style={{
                    backgroundColor: seatColor,
                  }}
                >
                  {<div css={styles.seatId} style={{ fontWeight: "bold" }}>
                    {eachSeat.seat_id}
                  </div>}
                  {workName !== '' && (<div css={styles.workName}>{workName}</div>)}
                  <div
                    css={styles.userDisplayName}
                  >
                    {displayName}
                  </div>

                </div>

              );
            })
          }

        </div>
    </div>
  );
}

export default DefaultRoomLayout;
