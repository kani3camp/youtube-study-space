import { RoomLayout } from '../types/room-layout'
import { classRoomLayout } from './layouts/classroom'
import { Freepick1RoomLayout } from './layouts/freepick1_room'
import { Freepick2RoomLayout } from './layouts/freepick2_room'
import { HimajinRoomLayout } from './layouts/himajin_room'
import { mazeRoomLayout } from './layouts/maze_room'
import { SimpleRoomLayout } from './layouts/simple_room'
import { ver2RoomLayout } from './layouts/ver2'

type AllRoomsConfig = {
    basicRooms: RoomLayout[]
    temporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
    basicRooms: [Freepick1RoomLayout, Freepick2RoomLayout],
    temporaryRooms: [
        classRoomLayout,
        SimpleRoomLayout,
        mazeRoomLayout,
        HimajinRoomLayout,
    ],
}

const testAllRooms: AllRoomsConfig = {
    basicRooms: [Freepick1RoomLayout, Freepick2RoomLayout, ver2RoomLayout],
    temporaryRooms: [Freepick1RoomLayout, Freepick2RoomLayout],
}

export const allRooms: AllRoomsConfig = testAllRooms

export const numSeatsInAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    for (const room of allRooms.basicRooms) {
        numSeatsBasicRooms += room.seats.length
    }
    return numSeatsBasicRooms
}
