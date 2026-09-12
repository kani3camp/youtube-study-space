import type { RoomLayout } from '../types/room-layout'
import { Anonymous1Room } from './layouts/anonymous1'
import { BookOfficeRoom } from './layouts/book-office-room'
import { CafeRainyRoom } from './layouts/cafe-rainy-room'
import { CampRoom } from './layouts/camp-room'
import { Chabio1Room } from './layouts/chabio1-room'
import { Chabio2Room } from './layouts/chabio2-room'
import { Freepik3Room } from './layouts/freepik3-room'
import { Freepik5Room } from './layouts/freepik5-room'
import { Freepik7Room } from './layouts/freepik7-room'
import { Freepik8Room } from './layouts/freepik8-room'
import { MemberBoxRooms2 } from './layouts/member-box-rooms-2'
import { MemberBoxRooms3 } from './layouts/member-box-rooms-3'
import { MemberIllustratedRoomSpring } from './layouts/member-illustrated-room-spring'
import { MemberIllustratedRoom1 } from './layouts/member-illustrated-room1'
import { MoonNightRoom1 } from './layouts/moon-night-room-1'
import { MoonNightRoom2 } from './layouts/moon-night-room-2'
import { ResortSeaRoom } from './layouts/resort-sea-room'

export const runtimeRoomRegistry = {
	chabio2: Chabio2Room,
	camp: CampRoom,
	cafeRainy: CafeRainyRoom,
	anonymous1: Anonymous1Room,
	freepik8: Freepik8Room,
	moonNight1: MoonNightRoom1,
	moonNight2: MoonNightRoom2,
	chabio1: Chabio1Room,
	freepik3: Freepik3Room,
	freepik5: Freepik5Room,
	freepik7: Freepik7Room,
	bookOffice: BookOfficeRoom,
	memberBoxRooms2: MemberBoxRooms2,
	memberBoxRooms3: MemberBoxRooms3,
	resortSea: ResortSeaRoom,
	memberIllustratedRoomSpring: MemberIllustratedRoomSpring,
	memberIllustratedRoom1: MemberIllustratedRoom1,
} as const satisfies Record<string, RoomLayout>

export type RuntimeRoomId = keyof typeof runtimeRoomRegistry
