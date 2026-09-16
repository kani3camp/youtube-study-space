import { Timestamp } from 'firebase/firestore'
import type { Seat } from '../types/api'

export const roomGallerySeatStates = [
	'Representative',
	'Empty',
	'Occupied',
] as const
export type RoomGallerySeatState = (typeof roomGallerySeatStates)[number]

const fixtureNow = Timestamp.fromMillis(Date.now())
const fixtureUntil = Timestamp.fromMillis(Date.now() + 45 * 60 * 1000)

const createSeat = (seatId: number, variant: number): Seat => {
	const isBreak = variant === 3
	const isLong = variant === 2
	return {
		seat_id: seatId,
		user_id: `room-gallery-user-${seatId}`,
		user_display_name: isLong
			? 'とても長い表示名のユーザーさん'
			: `Gallery User ${seatId}`,
		work_name: isLong
			? '長い作業名でもSeatBoxの中で読みやすく表示できるか確認する作業'
			: isBreak
				? 'コーヒー休憩中'
				: `作業サンプル ${seatId}`,
		entered_at: fixtureNow,
		until: fixtureUntil,
		appearance: {
			schema_version: 2,
			top_bar_color: variant % 2 === 0 ? '#5BD27D' : '#FF9B71',
			rank: Math.min(variant + 1, 10),
			rank_visible: variant === 4,
			num_stars: variant === 4 ? 5 : 0,
		},
		menu_code: variant === 4 ? 'coffee' : '',
		state: isBreak ? ('break' as Seat['state']) : ('work' as Seat['state']),
		current_state_started_at: fixtureNow,
		current_state_until: fixtureUntil,
		cumulative_work_sec: 60 * 60,
		daily_cumulative_work_sec: 60 * 60,
		user_profile_image_url: variant === 4 ? '/images/sample_profile.svg' : '',
	}
}

export const createRoomGallerySeats = (
	state: RoomGallerySeatState,
	seatCount: number,
): Seat[] => {
	if (state === 'Empty') {
		return []
	}

	const seatIds =
		state === 'Occupied'
			? Array.from({ length: seatCount }, (_, index) => index + 1)
			: [2, 3, 4, 5, 6].filter((seatId) => seatId <= seatCount)

	return seatIds.map((seatId, index) => createSeat(seatId, index + 1))
}
