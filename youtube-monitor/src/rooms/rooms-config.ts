import { RoomLayout } from "../types/room-layout";
import { circleRoomLayout } from "./layouts/circle_room";
import { classRoomLayout } from "./layouts/classroom";
import { HimajinRoomLayout } from "./layouts/himajin_room";
import { iLineRoomLayout } from "./layouts/iline_room";
import { mazeRoomLayout } from "./layouts/maze_room";
import { oneSeatRoomLayout } from "./layouts/one_seat_room";
import { SeaOfSeatRoomLayout } from "./layouts/sea_of_seat_room";
import { SimpleRoomLayout } from "./layouts/simple_room";
import { ver2RoomLayout } from "./layouts/ver2";

type AllRoomsConfig = {
  basicRooms: RoomLayout[];
  temporaryRooms: RoomLayout[];
};

const prodAllRooms: AllRoomsConfig = {
  basicRooms: [
    circleRoomLayout,
    mazeRoomLayout,
    HimajinRoomLayout,
    SeaOfSeatRoomLayout,
  ],
  temporaryRooms: [
    classRoomLayout,
    SimpleRoomLayout,
    mazeRoomLayout,
    HimajinRoomLayout,
  ],
};

const testAllRooms: AllRoomsConfig = {
  basicRooms: [circleRoomLayout, SimpleRoomLayout],
  temporaryRooms: [SimpleRoomLayout],
};

export const allRooms: AllRoomsConfig = testAllRooms;

export const numSeatsInAllBasicRooms = (): number => {
  let numSeatsBasicRooms = 0;
  for (const r of allRooms.basicRooms) {
    numSeatsBasicRooms += r.seats.length;
  }
  return numSeatsBasicRooms;
};
