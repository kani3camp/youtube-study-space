import type { RoomConfigName } from '../lib/constants'
import { type RoomId, roomRegistry } from './room-registry'
import { type RoomConfig, roomConfigs } from './rooms-config'

export const roomConfigCategoryOrder = [
	'generalBasicRooms',
	'memberBasicRooms',
	'generalTemporaryRooms',
	'memberTemporaryRooms',
] as const satisfies readonly (keyof RoomConfig)[]

export type RoomGalleryCategory = 'General' | 'Member'
export type RoomGalleryKind = 'Basic' | 'Temporary'

export type RoomGalleryEntry = {
	id: RoomId
	displayName: string
	layout: (typeof roomRegistry)[RoomId]['layout']
	seatCount: number
	enabled: boolean
	categories: readonly RoomGalleryCategory[]
	kinds: readonly RoomGalleryKind[]
}

export function getRoomGalleryEntries(
	configName: RoomConfigName,
): RoomGalleryEntry[] {
	const config = roomConfigs[configName]
	const categoryByRoom = new Map<RoomId, Set<RoomGalleryCategory>>()
	const kindByRoom = new Map<RoomId, Set<RoomGalleryKind>>()
	const configRankByRoom = new Map<RoomId, number>()
	let nextConfigRank = 0

	for (const category of roomConfigCategoryOrder) {
		for (const roomId of config[category]) {
			const categories = categoryByRoom.get(roomId) ?? new Set()
			categories.add(category.startsWith('general') ? 'General' : 'Member')
			categoryByRoom.set(roomId, categories)

			const kinds = kindByRoom.get(roomId) ?? new Set()
			kinds.add(category.endsWith('BasicRooms') ? 'Basic' : 'Temporary')
			kindByRoom.set(roomId, kinds)

			if (!configRankByRoom.has(roomId)) {
				configRankByRoom.set(roomId, nextConfigRank)
			}
			nextConfigRank += 1
		}
	}

	return (Object.keys(roomRegistry) as RoomId[])
		.map((id, registryIndex) => {
			const entry = roomRegistry[id]
			return {
				id,
				displayName: entry.displayName,
				layout: entry.layout,
				seatCount: entry.layout.seats.length,
				enabled: categoryByRoom.has(id),
				categories: [...(categoryByRoom.get(id) ?? [])],
				kinds: [...(kindByRoom.get(id) ?? [])],
				configRank:
					configRankByRoom.get(id) ??
					roomConfigCategoryOrder.length + registryIndex,
			}
		})
		.sort((a, b) => {
			if (a.enabled !== b.enabled) {
				return a.enabled ? -1 : 1
			}
			return a.configRank - b.configRank
		})
		.map(({ configRank: _configRank, ...entry }) => entry)
}
