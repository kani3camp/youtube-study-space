import { parseRoomConfigName, roomConfigNames } from '../lib/constants'
import { getRoomGalleryEntries, roomConfigCategoryOrder } from './room-gallery'
import { type RoomId, roomRegistry } from './room-registry'
import {
	type RoomConfig,
	roomConfigs,
	validateRoomConfig,
} from './rooms-config'

jest.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: jest.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: jest.fn(() => ({
		style: { fontFamily: 'mock-source-code-pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

test('every room config only refers to rooms in the registry', () => {
	for (const configName of roomConfigNames) {
		const config = roomConfigs[configName]
		for (const category of roomConfigCategoryOrder) {
			for (const roomId of config[category]) {
				expect(roomRegistry[roomId]).toBeDefined()
			}
		}
	}
})

test('room config names are validated at the environment boundary', () => {
	expect(parseRoomConfigName('PROD')).toBe('PROD')
	expect(parseRoomConfigName('DEV')).toBe('DEV')
	expect(() => parseRoomConfigName('staging')).toThrow(
		'invalid NEXT_PUBLIC_ROOM_CONFIG',
	)
})

test('room config categories do not contain duplicate room IDs', () => {
	const configWithDuplicate: RoomConfig = {
		...roomConfigs.PROD,
		generalBasicRooms: ['chabio2', 'chabio2'],
	}

	expect(() => validateRoomConfig(configWithDuplicate)).toThrow(
		'duplicate Room ID in generalBasicRooms',
	)
})

test.each(roomConfigNames)(
	'%s marks configured rooms as enabled',
	(configName) => {
		const entries = getRoomGalleryEntries(configName)
		const enabledIds = new Set<RoomId>(
			roomConfigCategoryOrder.flatMap(
				(category) => roomConfigs[configName][category],
			),
		)

		for (const entry of entries) {
			expect(entry.enabled).toBe(enabledIds.has(entry.id))
		}
	},
)

test('gallery entries include each registered room once and keep enabled rooms first', () => {
	const entries = getRoomGalleryEntries('PROD')

	expect(entries).toHaveLength(Object.keys(roomRegistry).length)
	expect(new Set(entries.map((entry) => entry.id)).size).toBe(entries.length)
	expect(entries.findIndex((entry) => !entry.enabled)).toBeGreaterThan(
		entries.findIndex((entry) => entry.enabled),
	)
})

test('gallery sorting follows category priority and registry order for other rooms', () => {
	const entries = getRoomGalleryEntries('PROD')
	const enabledIds = entries
		.filter((entry) => entry.enabled)
		.map((entry) => entry.id)
	const expectedEnabledIds = [
		'chabio2',
		'camp',
		'cafeRainy',
		'anonymous1',
		'freepik8',
		'moonNight1',
		'moonNight2',
		'bookOffice',
		'memberBoxRooms2',
		'memberBoxRooms3',
		'resortSea',
		'memberIllustratedRoomSpring',
		'memberIllustratedRoom1',
		'chabio1',
		'freepik3',
		'freepik5',
		'freepik7',
	]

	expect(enabledIds).toEqual(expectedEnabledIds)
})
