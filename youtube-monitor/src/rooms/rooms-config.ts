import { CHANNEL_GL, DEBUG } from '../lib/constants'
import { RoomLayout } from '../types/room-layout'
import { Anonymous1Room } from './layouts/anonymous1'
import { Chabio1Room } from './layouts/chabio1-room'
import { Chabio2Room } from './layouts/chabio2-room'
import { circleRoom } from './layouts/circle-room'
import { classRoom } from './layouts/classroom'
import { Freepik1Room } from './layouts/freepik1-room'
import { Freepik2Room } from './layouts/freepik2-room'
import { Freepik3Room } from './layouts/freepik3-room'
import { Freepik4Room } from './layouts/freepik4-room'
import { Freepik5Room } from './layouts/freepik5-room'
import { Freepik7Room } from './layouts/freepik7-room'
import { GLAkihabaraRoom } from './layouts/gl-in-akihabara'
import { GLOnlineRoom } from './layouts/gl-in-online-room'
import { mazeRoom } from './layouts/maze-room'
import { MemberBoxRooms2 } from './layouts/member-box-rooms-2'
import { MemberIllustratedRoomChristmas } from './layouts/member-illustrated-room-christmas'
import { MemberIllustratedRoom1 } from './layouts/member-illustrated-room1'
import { SeaOfSeatRoom } from './layouts/sea-of-seat-room'

type AllRoomsConfig = {
    generalBasicRooms: RoomLayout[]
    generalTemporaryRooms: RoomLayout[]
    memberBasicRooms: RoomLayout[]
    memberTemporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
    generalBasicRooms: [mazeRoom, Anonymous1Room, Freepik7Room, Freepik1Room, Freepik4Room],
    generalTemporaryRooms: [
        Chabio1Room,
        Freepik3Room,
        classRoom,
        Freepik2Room,
        Chabio2Room,
        circleRoom,
        Freepik5Room,
        SeaOfSeatRoom,
    ],
    memberBasicRooms: [MemberBoxRooms2, MemberIllustratedRoomChristmas],
    memberTemporaryRooms: [MemberIllustratedRoom1, MemberBoxRooms2, MemberIllustratedRoomChristmas],
}

const testAllRooms: AllRoomsConfig = {
    generalBasicRooms: [Freepik7Room, MemberBoxRooms2, MemberIllustratedRoomChristmas],
    generalTemporaryRooms: [
        Anonymous1Room,
        Chabio1Room,
        Freepik1Room,
        Freepik4Room,
        Freepik3Room,
        classRoom,
        Freepik2Room,
        Chabio2Room,
        circleRoom,
        Freepik5Room,
        SeaOfSeatRoom,
    ],
    memberBasicRooms: [MemberIllustratedRoom1, MemberIllustratedRoomChristmas],
    memberTemporaryRooms: [MemberIllustratedRoom1, MemberIllustratedRoomChristmas],
}

const glRooms: AllRoomsConfig = {
    generalBasicRooms: [GLOnlineRoom, GLAkihabaraRoom],
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
