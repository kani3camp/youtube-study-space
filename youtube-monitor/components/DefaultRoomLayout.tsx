import React, { useState, useEffect, useReducer, FC } from "react";
import * as styles from "./DefaultRoomLayout.styles";
import { RoomLayout } from "../types/room-layout";
import { Seat } from "../types/api";

type Props = {
  roomLayout: RoomLayout;
  seats: Seat[];
  firstSeatId: number;
}

const DefaultRoomLayout: FC<Props> = (props) => {
  const seatWithSeatId = (seatId: number, seats: Seat[]) => {
    let targetSeat: Seat = seats[0];
    seats.forEach((seat) => {
      if (seat.seat_id === seatId) {
        targetSeat = seat;
      }
    });
    return targetSeat;
  }

  if (props.roomLayout) {
    const roomSeats = props.seats
    const usedSeatIds = props.seats.map(
      (seat) => seat.seat_id
    );

    const emptySeatColor = "#F3E8DC";

    const roomLayout = props.roomLayout;
    const roomShape = {
      widthPx:
        (1000 * roomLayout.room_shape.width) / roomLayout.room_shape.height,
      heightPx: 1000,
    };

    const seatFontSizePx = roomShape.widthPx * roomLayout.font_size_ratio;

    const seatShape = {
      width:
        (100 * roomLayout.seat_shape.width) / roomLayout.room_shape.width,
      height:
        (100 * roomLayout.seat_shape.height) / roomLayout.room_shape.height,
    };

    const seatPositions = roomLayout.seats.map((seat) => ({
      x: (100 * seat.x) / roomLayout.room_shape.width,
      y: (100 * seat.y) / roomLayout.room_shape.height,
    }));

    const partitionShapes = roomLayout.partitions.map((partition) => {
      const partitionShapes = roomLayout.partition_shapes;
      const shapeType = partition.shape_type;
      let widthPercent;
      let heightPercent;
      for (let i = 0; i < partitionShapes.length; i++) {
        if (partitionShapes[i].name === shapeType) {
          widthPercent =
            (100 * partitionShapes[i].width) / roomLayout.room_shape.width;
          heightPercent =
            (100 * partitionShapes[i].height) / roomLayout.room_shape.height;
        }
      }
      return {
        widthPercent,
        heightPercent,
      };
    });

    const partitionPositions = roomLayout.partitions.map((partition) => ({
      x: (100 * partition.x) / roomLayout.room_shape.width,
      y: (100 * partition.y) / roomLayout.room_shape.height,
    }));

    const seatList = props.roomLayout.seats.map((seat, index) => {
      const isUsed = usedSeatIds.includes(seat.id);
      const workName = usedSeatIds.includes(seat.id)
        ? seatWithSeatId(seat.id, props.seats).work_name
        : "";
      const displayName = usedSeatIds.includes(seat.id)
        ? seatWithSeatId(seat.id, props.seats)
          .user_display_name
        : "";
      const seatColor = roomSeats.find(s => s.seat_id === seat.id)?.color_code;
      return (
        <div
          key={seat.id}
          css={styles.seat}
          style={{
            backgroundColor: isUsed ? seatColor : emptySeatColor,
            left: seatPositions[index].x + "%",
            top: seatPositions[index].y + "%",
            width: seatShape.width + "%",
            height: seatShape.height + "%",
            fontSize: seatFontSizePx + "px",
          }}
        >
          <div css={styles.seatId} style={{ fontWeight: "bold" }}>
            {seat.id}
          </div>
          {workName !== '' && (<div css={styles.workName}>{workName}</div>)}
          <div
            css={styles.userDisplayName}
          >
            {displayName}
          </div>
        </div>
      );
    });
    

    const partitionList = roomLayout.partitions.map((partition, index) => (
      <div
        key={partition.id}
        css={styles.partition}
        style={{
          left: partitionPositions[index].x + "%",
          top: partitionPositions[index].y + "%",
          width: partitionShapes[index].widthPercent + "%",
          height: partitionShapes[index].heightPercent + "%",
        }}
      />
    ));

    return (
      <div
        css={styles.roomLayout}
        style={{
          width: roomShape.widthPx + "px",
          height: roomShape.heightPx + "px",
        }}
      >
        {seatList}

        {partitionList}
      </div>
    );
  } else {
    return <div>Loading</div>;
  }

}

export default DefaultRoomLayout;
