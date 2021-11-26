import { RoomLayout } from "../types/room-layout";
import { room1layout } from "./layouts/room1";

type RoomsConfig = {
    roomLayouts: RoomLayout[];

}

export const basicRooms: RoomsConfig = {
    roomLayouts: [room1layout]
}

export const temporaryRooms: RoomsConfig = {
    roomLayouts: []
}


export const numSeatsBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    basicRooms.roomLayouts.forEach(r => {
        numSeatsBasicRooms += r.seats.length
    })
    return numSeatsBasicRooms
}

