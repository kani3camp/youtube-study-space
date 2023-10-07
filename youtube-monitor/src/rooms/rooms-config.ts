import { CHANNEL_GL, DEBUG } from '../lib/constants'
import { RoomLayout } from '../types/room-layout'
import { Anonymous1Layout } from './layouts/anonymous1'
import { Chabio1Layout } from './layouts/chabio1-room'
import { Chabio2Layout } from './layouts/chabio2-room'
import { circleRoomLayout } from './layouts/circle-room'
import { classRoomLayout } from './layouts/classroom'
import { Freepik1RoomLayout } from './layouts/freepik1-room'
import { Freepik2RoomLayout } from './layouts/freepik2-room'
import { Freepik3RoomLayout } from './layouts/freepik3-room'
import { Freepik4Layout } from './layouts/freepik4-room'
import { Freepik5Layout } from './layouts/freepik5-room'
import { Freepik6Layout } from './layouts/freepik6-room'
import { GLAkihabaraLayout } from './layouts/gl-in-akihabara'
import { GLOnlineLayout } from './layouts/gl-in-online-room'
import { mazeRoomLayout } from './layouts/maze-room'
import { MemberIllustratedRoom1 } from './layouts/member-illustrated-room1'
import { MemberIllustratedRoom2Beach } from './layouts/member-illustrated-room2-beach'
import { SeaOfSeatRoomLayout } from './layouts/sea-of-seat-room'

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
        Freepik6Layout,
        Freepik1RoomLayout,
        Freepik4Layout,
    ],
    generalTemporaryRooms: [
        Chabio1Layout,
        Freepik3RoomLayout,
        classRoomLayout,
        Freepik2RoomLayout,
        Chabio2Layout,
        circleRoomLayout,
        Freepik5Layout,
        SeaOfSeatRoomLayout,
    ],
    memberBasicRooms: [MemberIllustratedRoom1, MemberIllustratedRoom2Beach],
    memberTemporaryRooms: [MemberIllustratedRoom1, MemberIllustratedRoom2Beach],
}

const testAllRooms: AllRoomsConfig = {
    generalBasicRooms: [mazeRoomLayout, Freepik6Layout],
    generalTemporaryRooms: [
        Anonymous1Layout,
        Chabio1Layout,
        Freepik1RoomLayout,
        Freepik4Layout,
        Freepik3RoomLayout,
        classRoomLayout,
        Freepik2RoomLayout,
        Chabio2Layout,
        circleRoomLayout,
        Freepik5Layout,
        SeaOfSeatRoomLayout,
    ],
    memberBasicRooms: [MemberIllustratedRoom1, MemberIllustratedRoom2Beach],
    memberTemporaryRooms: [MemberIllustratedRoom1, MemberIllustratedRoom2Beach],
}

const glRooms: AllRoomsConfig = {
    generalBasicRooms: [GLOnlineLayout, GLAkihabaraLayout],
    generalTemporaryRooms: [],
    memberBasicRooms: [],
    memberTemporaryRooms: [],
}

export const allRooms: AllRoomsConfig = CHANNEL_GL ? glRooms : DEBUG ? testAllRooms : prodAllRooms

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
