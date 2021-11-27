import { RoomLayout } from "../types/room-layout";
import { classRoomLayout } from "./layouts/classroom";
import { ver2RoomLayout } from "./layouts/ver2";

type RoomsConfig = {
    roomLayouts: RoomLayout[];

}

export const basicRooms: RoomsConfig = {
    roomLayouts: [ver2RoomLayout]
}

export const temporaryRooms: RoomsConfig = {
    roomLayouts: [classRoomLayout]
}


export const numSeatsInAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    basicRooms.roomLayouts.forEach(r => {
        numSeatsBasicRooms += r.seats.length
    })
    return numSeatsBasicRooms
}

