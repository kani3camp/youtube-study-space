import { DEBUG } from '../lib/constants'
import { RoomLayout } from '../types/room-layout'
import { Anonymous1Layout } from './layouts/anonymous1'
import { Chabio1Layout } from './layouts/chabio1_room'
import { Chabio2Layout } from './layouts/chabio2_room'
import { circleRoomLayout } from './layouts/circle_room'
import { classRoomLayout } from './layouts/classroom'
import { Freepik1RoomLayout } from './layouts/freepik1_room'
import { Freepik2RoomLayout } from './layouts/freepik2_room'
import { Freepik3RoomLayout } from './layouts/freepik3_room'
import { Freepik4Layout } from './layouts/freepik4_room'
import { Freepik5Layout } from './layouts/freepik5_room'
import { mazeRoomLayout } from './layouts/maze_room'
import { MemberIllustratedRoom1 } from './layouts/member_illustrated_room1'
import { MemberSimpleRoom1 } from './layouts/member_simple_room1'
import { SeaOfSeatRoomLayout } from './layouts/sea_of_seat_room'

type AllRoomsConfig = {
    generalBasicRooms: RoomLayout[]
    generalTemporaryRooms: RoomLayout[]
    memberBasicRooms: RoomLayout[]
    memberTemporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
    generalBasicRooms: [
        mazeRoomLayout,
        Anonymous1Layout,
        Chabio1Layout,
        Freepik1RoomLayout,
        Freepik3RoomLayout,
        Freepik5Layout,
    ],
    generalTemporaryRooms: [
        classRoomLayout,
        Freepik2RoomLayout,
        Freepik4Layout,
        Chabio2Layout,
        Freepik1RoomLayout,
        circleRoomLayout,
        Freepik5Layout,
    ],
    memberBasicRooms: [MemberIllustratedRoom1],
    memberTemporaryRooms: [MemberSimpleRoom1],
}

const testAllRooms: AllRoomsConfig = {
    generalBasicRooms: [mazeRoomLayout],
    generalTemporaryRooms: [
        SeaOfSeatRoomLayout,
        Freepik2RoomLayout,
        Freepik4Layout,
        Chabio2Layout,
        Freepik1RoomLayout,
        Freepik3RoomLayout,
        Freepik5Layout,
    ],
    memberBasicRooms: [MemberIllustratedRoom1],
    memberTemporaryRooms: [MemberSimpleRoom1],
}

export const allRooms: AllRoomsConfig = DEBUG ? testAllRooms : prodAllRooms

export const numSeatsInGeneralAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    for (const room of allRooms.generalBasicRooms) {
        numSeatsBasicRooms += room.seats.length
    }
    return numSeatsBasicRooms
}
export const numSeatsInMemberAllBasicRooms = (): number => {
    let numSeatsBasicRooms = 0
    for (const room of allRooms.memberBasicRooms) {
        numSeatsBasicRooms += room.seats.length
    }
    return numSeatsBasicRooms
}
