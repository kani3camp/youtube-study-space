import { ROOM_CONFIG, type RoomConfigName } from '../lib/constants'
import type { RoomLayout } from '../types/room-layout'
import { type RoomId, roomRegistry } from './room-registry'

export type RoomConfig = {
	readonly generalBasicRooms: readonly RoomId[]
	readonly generalTemporaryRooms: readonly RoomId[]
	readonly memberBasicRooms: readonly RoomId[]
	readonly memberTemporaryRooms: readonly RoomId[]
}

export type ResolvedRoomConfig = {
	readonly [K in keyof RoomConfig]: RoomLayout[]
}

export const roomConfigs = {
	PROD: {
		generalBasicRooms: [
			'chabio2',
			'camp',
			'cafeRainy',
			'anonymous1',
			'freepik8',
			'moonNight1',
			'moonNight2',
		],
		generalTemporaryRooms: [
			'camp',
			'chabio1',
			'freepik3',
			'cafeRainy',
			'freepik5',
			'freepik8',
			'freepik7',
			'moonNight1',
			'moonNight2',
		],
		memberBasicRooms: [
			'bookOffice',
			'memberBoxRooms2',
			'memberBoxRooms3',
			'memberIllustratedRoomSpring',
			'memberIllustratedRoom1',
		],
		memberTemporaryRooms: [
			'bookOffice',
			'memberBoxRooms2',
			'memberBoxRooms3',
			'memberIllustratedRoomSpring',
			'memberIllustratedRoom1',
		],
	},
	DEV: {
		generalBasicRooms: ['moonNight1', 'moonNight2'],
		generalTemporaryRooms: ['moonNight1', 'moonNight2'],
		memberBasicRooms: ['bookOffice'],
		memberTemporaryRooms: ['bookOffice'],
	},
} as const satisfies Record<RoomConfigName, RoomConfig>

const roomConfigCategories = [
	'generalBasicRooms',
	'memberBasicRooms',
	'generalTemporaryRooms',
	'memberTemporaryRooms',
] as const satisfies readonly (keyof RoomConfig)[]

export function validateRoomConfig(config: RoomConfig): void {
	for (const category of roomConfigCategories) {
		const roomIds = config[category]
		if (new Set(roomIds).size !== roomIds.length) {
			throw new Error(`duplicate Room ID in ${category}`)
		}
		for (const roomId of roomIds) {
			if (!(roomId in roomRegistry)) {
				throw new Error(`unknown Room ID in ${category}: ${roomId}`)
			}
		}
	}
}

for (const config of Object.values(roomConfigs)) {
	validateRoomConfig(config)
}

export const resolveRoomConfig = (config: RoomConfig): ResolvedRoomConfig => {
	validateRoomConfig(config)
	return {
		generalBasicRooms: config.generalBasicRooms.map(
			(roomId) => roomRegistry[roomId].layout,
		),
		generalTemporaryRooms: config.generalTemporaryRooms.map(
			(roomId) => roomRegistry[roomId].layout,
		),
		memberBasicRooms: config.memberBasicRooms.map(
			(roomId) => roomRegistry[roomId].layout,
		),
		memberTemporaryRooms: config.memberTemporaryRooms.map(
			(roomId) => roomRegistry[roomId].layout,
		),
	}
}

export const allRooms = resolveRoomConfig(roomConfigs[ROOM_CONFIG])

export const numSeatsInGeneralAllBasicRooms = (): number =>
	allRooms.generalBasicRooms.reduce(
		(count, room) => count + room.seats.length,
		0,
	)

export const numSeatsInMemberAllBasicRooms = (): number =>
	allRooms.memberBasicRooms.reduce(
		(count, room) => count + room.seats.length,
		0,
	)
