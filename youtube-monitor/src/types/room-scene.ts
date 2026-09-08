export const ROOM_SCENE_MODES = ['ambient', 'living'] as const

export type RoomSceneMode = (typeof ROOM_SCENE_MODES)[number]

export const LIVING_SCENE_PROFILES = ['lume-rainy-poc'] as const
export type LivingSceneProfile = (typeof LIVING_SCENE_PROFILES)[number]

export type AmbientRoomSceneConfig = {
	mode: 'ambient'
	profile?: string
}

export type LivingRoomSceneConfig = {
	mode: 'living'
	profile: LivingSceneProfile
}

/**
 * Optional visual enhancement for a room.
 *
 * Omitting scene keeps the room fully static. Living profiles are explicit so
 * an unknown/missing profile cannot silently start an empty WebGL renderer.
 */
export type RoomSceneConfig = AmbientRoomSceneConfig | LivingRoomSceneConfig
