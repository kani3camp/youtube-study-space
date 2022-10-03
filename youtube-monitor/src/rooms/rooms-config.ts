import { debug } from '../lib/constants'
import { RoomLayout } from '../types/room-layout'
import { Anonymous1Layout } from './layouts/anonymous1'
import { Chabio1Layout } from './layouts/chabio1_room'
import { Chabio2Layout } from './layouts/chabio2_room'
import { Freepik1RoomLayout } from './layouts/freepik1_room'
import { Freepik2RoomLayout } from './layouts/freepik2_room'
import { Freepik3RoomLayout } from './layouts/freepik3_room'
import { Freepik4Layout } from './layouts/freepik4_room'
import { Freepik5Layout } from './layouts/freepik5_room'
import { mazeRoomLayout } from './layouts/maze_room'
import { SeaOfSeatRoomLayout } from './layouts/sea_of_seat_room'

type AllRoomsConfig = {
    basicRooms: RoomLayout[]
    temporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
    basicRooms: [
        mazeRoomLayout,
        Anonymous1Layout,
        Chabio1Layout,
        Freepik1RoomLayout,
        Freepik3RoomLayout,
        Freepik5Layout,
    ],
    temporaryRooms: [
        SeaOfSeatRoomLayout,
        Freepik2RoomLayout,
        Freepik4Layout,
        Chabio2Layout,
        Freepik1RoomLayout,
        Freepik3RoomLayout,
        Freepik5Layout,
    ],
}

const testAllRooms: AllRoomsConfig = {
    basicRooms: [
        mazeRoomLayout,
        Anonymous1Layout,
        Chabio1Layout,
        Freepik1RoomLayout,
        Freepik3RoomLayout,
        Freepik5Layout,
    ],
    temporaryRooms: [
        SeaOfSeatRoomLayout,
        Freepik2RoomLayout,
        Freepik4Layout,
        Chabio2Layout,
        Freepik1RoomLayout,
        Freepik3RoomLayout,
        Freepik5Layout,
    ],
}

export const allRooms: AllRoomsConfig = debug ? testAllRooms : prodAllRooms

export const numSeatsInAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    for (const room of allRooms.basicRooms) {
        numSeatsBasicRooms += room.seats.length
    }
    return numSeatsBasicRooms
}
