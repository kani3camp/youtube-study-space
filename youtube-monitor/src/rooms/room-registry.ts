import type { RoomLayout } from '../types/room-layout'
import { Anonymous1Room } from './layouts/anonymous1'
import { BookOfficeRoom } from './layouts/book-office-room'
import { BookOfficeRoom2 } from './layouts/book-office-room-2'
import { CafeRainyRoom } from './layouts/cafe-rainy-room'
import { CafeWinterNightRoom } from './layouts/cafe-winter-night-room'
import { CampRoom } from './layouts/camp-room'
import { Chabio1Room } from './layouts/chabio1-room'
import { Chabio2Room } from './layouts/chabio2-room'
import { circleRoom } from './layouts/circle-room'
import { CityNightViewRoom } from './layouts/city-night-view-room'
import { classRoom } from './layouts/classroom'
import { Freepik1Room } from './layouts/freepik1-room'
import { Freepik2Room } from './layouts/freepik2-room'
import { Freepik3Room } from './layouts/freepik3-room'
import { Freepik4Room } from './layouts/freepik4-room'
import { Freepik5Room } from './layouts/freepik5-room'
import { Freepik6Room } from './layouts/freepik6-room'
import { Freepik7Room } from './layouts/freepik7-room'
import { Freepik8Room } from './layouts/freepik8-room'
import { HimajinRoom } from './layouts/himajin-room'
import { iLineRoom } from './layouts/iline-room'
import { mazeRoom } from './layouts/maze-room'
import { MemberBoxRooms2 } from './layouts/member-box-rooms-2'
import { MemberBoxRooms3 } from './layouts/member-box-rooms-3'
import { MemberIllustratedRoomChristmas } from './layouts/member-illustrated-room-christmas'
import { MemberIllustratedRoomSpring } from './layouts/member-illustrated-room-spring'
import { MemberIllustratedRoom1 } from './layouts/member-illustrated-room1'
import { MemberIllustratedRoom2Beach } from './layouts/member-illustrated-room2-beach'
import { MemberIllustratedRoom2Halloween } from './layouts/member-illustrated-room2-halloween'
import { MemberSimpleRoom1 } from './layouts/member-simple-room1'
import { MoonNightRoom1 } from './layouts/moon-night-room-1'
import { MoonNightRoom2 } from './layouts/moon-night-room-2'
import { oneSeatRoom } from './layouts/one-seat-room'
import { OtomeGameCafeRoom1 } from './layouts/otome-game-cafe-room-1'
import { OtomeGameCafeRoom2 } from './layouts/otome-game-cafe-room-2'
import { ResortSeaRoom } from './layouts/resort-sea-room'
import { SeaOfSeatRoom } from './layouts/sea-of-seat-room'
import { SimpleRoom } from './layouts/simple-room'
import { takochanRoom } from './layouts/takochan-room'
import { LayoutName } from './layouts/template'
import { ver2Room } from './layouts/ver2'

export const roomRegistry = {
	chabio2: { displayName: 'Chabio 2', layout: Chabio2Room },
	camp: { displayName: 'Camp', layout: CampRoom },
	cafeRainy: { displayName: 'Cafe Rainy', layout: CafeRainyRoom },
	anonymous1: { displayName: 'Anonymous 1', layout: Anonymous1Room },
	freepik8: { displayName: 'Freepik 8', layout: Freepik8Room },
	moonNight1: { displayName: 'Moon Night 1', layout: MoonNightRoom1 },
	moonNight2: { displayName: 'Moon Night 2', layout: MoonNightRoom2 },
	chabio1: { displayName: 'Chabio 1', layout: Chabio1Room },
	freepik3: { displayName: 'Freepik 3', layout: Freepik3Room },
	freepik5: { displayName: 'Freepik 5', layout: Freepik5Room },
	freepik7: { displayName: 'Freepik 7', layout: Freepik7Room },
	bookOffice: { displayName: 'Book Office', layout: BookOfficeRoom },
	bookOffice2: { displayName: 'Book Office 2', layout: BookOfficeRoom2 },
	memberBoxRooms2: {
		displayName: 'Member Box Rooms 2',
		layout: MemberBoxRooms2,
	},
	memberBoxRooms3: {
		displayName: 'Member Box Rooms 3',
		layout: MemberBoxRooms3,
	},
	resortSea: { displayName: 'Resort Sea', layout: ResortSeaRoom },
	memberIllustratedRoomSpring: {
		displayName: 'Member Illustrated Spring',
		layout: MemberIllustratedRoomSpring,
	},
	memberIllustratedRoom1: {
		displayName: 'Member Illustrated 1',
		layout: MemberIllustratedRoom1,
	},
	cafeWinterNight: {
		displayName: 'Cafe Winter Night',
		layout: CafeWinterNightRoom,
	},
	cityNightView: { displayName: 'City Night View', layout: CityNightViewRoom },
	freepik1: { displayName: 'Freepik 1', layout: Freepik1Room },
	freepik2: { displayName: 'Freepik 2', layout: Freepik2Room },
	freepik4: { displayName: 'Freepik 4', layout: Freepik4Room },
	freepik6: { displayName: 'Freepik 6', layout: Freepik6Room },
	circle: { displayName: 'Circle', layout: circleRoom },
	classroom: { displayName: 'Classroom', layout: classRoom },
	himajin: { displayName: 'Himajin', layout: HimajinRoom },
	iLine: { displayName: 'iLine', layout: iLineRoom },
	maze: { displayName: 'Maze', layout: mazeRoom },
	memberIllustratedRoomChristmas: {
		displayName: 'Member Illustrated Christmas',
		layout: MemberIllustratedRoomChristmas,
	},
	memberIllustratedRoom2Beach: {
		displayName: 'Member Illustrated 2 Beach',
		layout: MemberIllustratedRoom2Beach,
	},
	memberIllustratedRoom2Halloween: {
		displayName: 'Member Illustrated 2 Halloween',
		layout: MemberIllustratedRoom2Halloween,
	},
	memberSimpleRoom1: {
		displayName: 'Member Simple 1',
		layout: MemberSimpleRoom1,
	},
	oneSeat: { displayName: 'One Seat', layout: oneSeatRoom },
	otomeGameCafe1: {
		displayName: 'Otome Game Cafe 1',
		layout: OtomeGameCafeRoom1,
	},
	otomeGameCafe2: {
		displayName: 'Otome Game Cafe 2',
		layout: OtomeGameCafeRoom2,
	},
	seaOfSeat: { displayName: 'Sea of Seat', layout: SeaOfSeatRoom },
	simple: { displayName: 'Simple', layout: SimpleRoom },
	takochan: { displayName: 'Takochan', layout: takochanRoom },
	template: { displayName: 'Template', layout: LayoutName },
	ver2: { displayName: 'Ver. 2', layout: ver2Room },
} as const satisfies Record<
	string,
	{
		displayName: string
		layout: RoomLayout
	}
>

export type RoomId = keyof typeof roomRegistry
