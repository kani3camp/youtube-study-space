import type { RoomLayout } from '../../types/room-layout'

export type RuntimeProfile = 'general' | 'member'

export type RuntimeProfileIntent = RuntimeProfile | 'both'

export type Point = {
	x: number
	y: number
}

export type OverlayZone = {
	id: string
	label: string
	shape:
		| {
				type: 'rect'
				x: number
				y: number
				width: number
				height: number
		  }
		| {
				type: 'polygon'
				points: Point[]
		  }
}

/**
 * RoomLayout itself is the implementation-ready Seat Overlay Map. Review-only
 * metadata is attached to that same object so seat coordinates are never copied
 * between preview, validation, and production layout definitions.
 */
export type RuntimeRoomOverlayMap = RoomLayout & {
	runtime_preview: {
		id: string
		name: string
		description: string
		runtime_profile: RuntimeProfileIntent
		camera_profile: string
		protected_visual_zones: OverlayZone[]
		seat_zones: OverlayZone[]
		main_circulation: OverlayZone[]
		seat_zone_by_seat_id?: Record<number, string>
	}
}

export const runtimeProfiles = {
	general: {
		label: '一般席 140 × 100',
		seatShape: { width: 140, height: 100 },
		fontSizeRatio: 0.015,
		memberOnly: false,
	},
	member: {
		label: 'メンバー席 230 × 150',
		seatShape: { width: 230, height: 150 },
		fontSizeRatio: 0.017,
		memberOnly: true,
	},
} as const satisfies Record<
	RuntimeProfile,
	{
		label: string
		seatShape: RoomLayout['seat_shape']
		fontSizeRatio: number
		memberOnly: boolean
	}
>

export function roomLayoutForProfile(
	overlayMap: RuntimeRoomOverlayMap,
	profile: RuntimeProfile,
): RoomLayout {
	const runtimeProfile = runtimeProfiles[profile]
	return {
		...overlayMap,
		seat_shape: runtimeProfile.seatShape,
		font_size_ratio: runtimeProfile.fontSizeRatio,
	}
}
