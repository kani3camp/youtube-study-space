import { RoomLayout } from "../types/room-layout";
import { room1layout } from "./layouts/room1";

type RoomsConfig = {
    roomLayouts: RoomLayout[];

}

export const basicRooms: RoomsConfig = {
    roomLayouts: [room1layout]
}