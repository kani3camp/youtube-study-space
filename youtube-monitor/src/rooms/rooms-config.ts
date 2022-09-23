import { RoomLayout } from '../types/room-layout'
import { classRoomLayout } from './layouts/classroom'
import { Freepik1RoomLayout } from './layouts/freepik1_room'
import { Freepik2RoomLayout } from './layouts/freepik2_room'
import { Freepik3RoomLayout } from './layouts/freepik3_room'
import { HimajinRoomLayout } from './layouts/himajin_room'
import { mazeRoomLayout } from './layouts/maze_room'
import { SimpleRoomLayout } from './layouts/simple_room'

type AllRoomsConfig = {
    basicRooms: RoomLayout[]
    temporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
    basicRooms: [Freepik1RoomLayout, Freepik2RoomLayout],
    temporaryRooms: [
        classRoomLayout,
        SimpleRoomLayout,
        mazeRoomLayout,
        HimajinRoomLayout,
    ],
}

const testAllRooms: AllRoomsConfig = {
    basicRooms: [Freepik3RoomLayout],
    temporaryRooms: [
        Freepik1RoomLayout,
        Freepik2RoomLayout,
        Freepik3RoomLayout,
    ],
}

export const allRooms: AllRoomsConfig = testAllRooms

export const numSeatsInAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    for (const room of allRooms.basicRooms) {
        numSeatsBasicRooms += room.seats.length
    }
    return numSeatsBasicRooms
}
