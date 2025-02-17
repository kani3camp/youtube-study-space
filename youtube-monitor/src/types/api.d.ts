import type { Timestamp } from 'firebase/firestore'

export type SeatAppearance = {
	color_code1: string
	color_code2: string
	num_stars: number
	color_gradient_enabled: boolean
}

export type Seat = {
	seat_id: number
	user_id: string
	user_display_name: string
	work_name: string
	break_work_name: string
	entered_at: Timestamp
	until: Timestamp
	appearance: SeatAppearance
	menu_code: string
	state: SeatState
	current_state_started_at: Timestamp
	current_state_until: Timestamp
	cumulative_work_sec: number
	daily_cumulative_work_sec: number
	user_profile_image_url: string
}

export type Menu = {
	code: string
	name: string
}

export type SetDesiredMaxSeatsResponse = {
	result: string
	message: string
}
