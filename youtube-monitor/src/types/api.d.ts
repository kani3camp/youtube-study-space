import { Timestamp } from 'firebase/firestore'

export type SeatAppearance = {
    color_code: string
    num_stars: number
    glow_animation: boolean
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
    state: SeatState
    current_state_started_at: Timestamp
    current_state_until: Timestamp
    cumulative_work_sec: number
    daily_cumulative_work_sec: number
    user_profile_image_url: string
}

export type SetDesiredMaxSeatsResponse = {
    result: string
    message: string
}
