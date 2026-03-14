import { ROOM_CONFIG } from '../lib/constants'
import type { RoomLayout } from '../types/room-layout'
import { Anonymous1Room } from './layouts/anonymous1'
import { CafeRainyRoom } from './layouts/cafe-rainy-room'
import { CampRoom } from './layouts/camp-room'
import { Chabio1Room } from './layouts/chabio1-room'
import { Chabio2Room } from './layouts/chabio2-room'
import { Freepik3Room } from './layouts/freepik3-room'
import { Freepik4Room } from './layouts/freepik4-room'
import { Freepik5Room } from './layouts/freepik5-room'
import { Freepik6Room } from './layouts/freepik6-room'
import { Freepik7Room } from './layouts/freepik7-room'
import { Freepik8Room } from './layouts/freepik8-room'
import { MemberBoxRooms2 } from './layouts/member-box-rooms-2'
import { MemberBoxRooms3 } from './layouts/member-box-rooms-3'
import { MemberIllustratedRoom1 } from './layouts/member-illustrated-room1'
import { ResortSeaRoom } from './layouts/resort-sea-room'

type AllRoomsConfig = {
	generalBasicRooms: RoomLayout[]
	generalTemporaryRooms: RoomLayout[]
	memberBasicRooms: RoomLayout[]
	memberTemporaryRooms: RoomLayout[]
}

const prodAllRooms: AllRoomsConfig = {
	generalBasicRooms: [
		Chabio2Room,
		CampRoom,
		Freepik6Room,
		Anonymous1Room,
		Freepik8Room,
	],
	generalTemporaryRooms: [
		CampRoom,
		Chabio1Room,
		Freepik3Room,
		Freepik5Room,
		Freepik8Room,
		Freepik7Room,
	],
	memberBasicRooms: [MemberBoxRooms2, MemberBoxRooms3, ResortSeaRoom],
	memberTemporaryRooms: [MemberBoxRooms2, MemberBoxRooms3, ResortSeaRoom],
}

const testAllRooms: AllRoomsConfig = {
	generalBasicRooms: [CampRoom, Freepik8Room],
	generalTemporaryRooms: [CafeRainyRoom, CampRoom, Freepik8Room],
	memberBasicRooms: [ResortSeaRoom],
	memberTemporaryRooms: [ResortSeaRoom],
}

export const allRooms: AllRoomsConfig = (function getAllRooms() {
	switch (ROOM_CONFIG) {
		case 'PROD':
			return prodAllRooms
		case 'DEV':
			return testAllRooms
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
