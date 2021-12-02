import { RoomLayout } from "../types/room-layout";
import { classRoomLayout } from "./layouts/classroom";
import { ver2RoomLayout } from "./layouts/ver2";

type RoomsConfig = {
    roomLayouts: RoomLayout[];

}

export const basicRooms: RoomsConfig = {
    roomLayouts: [classRoomLayout]
}

export const temporaryRooms: RoomsConfig = {
    roomLayouts: [ver2RoomLayout, classRoomLayout]
}


export const numSeatsInAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    for (const r of basicRooms.roomLayouts) {
        numSeatsBasicRooms += r.seats.length
    }
    return numSeatsBasicRooms
}

