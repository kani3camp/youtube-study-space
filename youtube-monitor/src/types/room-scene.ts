export const ROOM_SCENE_MODES = ['ambient', 'living'] as const

export type RoomSceneMode = (typeof ROOM_SCENE_MODES)[number]

/**
 * Optional visual enhancement for a room.
 *
 * Omitting scene keeps the room fully static. The schema is intentionally
 * minimal here; renderer/layer details are introduced only when their runtime
 * contracts are implemented by later Live Room Scenes PRs.
 */
export type RoomSceneConfig = {
	mode: RoomSceneMode
	profile?: string
}
