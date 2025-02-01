import { ROOM_CONFIG } from '../lib/constants'
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
import { HimajinRoom } from './layouts/himajin-room'
import { MemberBoxRooms2 } from './layouts/member-box-rooms-2'
import { MemberIllustratedRoomChristmas } from './layouts/member-illustrated-room-christmas'
import { MemberIllustratedRoom1 } from './layouts/member-illustrated-room1'
import { oneSeatRoom } from './layouts/one-seat-room'
import { OtomeGameCafeRoom1 } from './layouts/otome-game-cafe-room-1'
import { OtomeGameCafeRoom2 } from './layouts/otome-game-cafe-room-2'
import { SeaOfSeatRoom } from './layouts/sea-of-seat-room'
import { takochanRoom } from './layouts/takochan-room'
import { ver2Room } from './layouts/ver2'

type AllRoomsConfig = {
    generalBasicRooms: RoomLayout[]
    generalTemporaryRooms: RoomLayout[]
    memberBasicRooms: RoomLayout[]
    memberTemporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
    generalBasicRooms: [Chabio2Room, Freepik7Room, Freepik1Room, Freepik4Room],
    generalTemporaryRooms: [Chabio1Room, Freepik3Room, Freepik2Room, Chabio2Room, Freepik5Room],
    memberBasicRooms: [MemberBoxRooms2, MemberIllustratedRoomChristmas, MemberIllustratedRoom1],
    memberTemporaryRooms: [MemberBoxRooms2, MemberIllustratedRoomChristmas, MemberIllustratedRoom1],
}

const testAllRooms: AllRoomsConfig = {
    generalBasicRooms: [
        Chabio2Room,
        Freepik7Room,
        circleRoom,
        HimajinRoom,
        classRoom,
        ver2Room,
        takochanRoom,
        SeaOfSeatRoom,
        oneSeatRoom,
    ],
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
    memberBasicRooms: [],
    memberTemporaryRooms: [],
}

const otomeGameCafeRooms: AllRoomsConfig = {
    generalBasicRooms: [OtomeGameCafeRoom1, OtomeGameCafeRoom2],
    generalTemporaryRooms: [
        Anonymous1Room,
        Chabio1Room,
        Freepik1Room,
        Freepik4Room,
        Freepik3Room,
        Freepik2Room,
        Chabio2Room,
        Freepik5Room,
    ],
    memberBasicRooms: [],
    memberTemporaryRooms: [],
}

export const allRooms: AllRoomsConfig = (function getAllRooms() {
    switch (ROOM_CONFIG) {
        case 'PROD':
            return prodAllRooms
        case 'DEV':
            return testAllRooms
        case 'OTOME-GAME-CAFE':
            return otomeGameCafeRooms
        default:
            throw new Error(`unknown ROOM_CONFIG: ${ROOM_CONFIG}`)
    }
})()

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
