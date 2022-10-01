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
    entered_at: Date
    until: Date
    appearance: SeatAppearance
    state: SeatState
    current_state_started_at: Date
    current_state_until: Date
    cumulative_work_sec: number
    daily_cumulative_work_sec: number
}

export type SetDesiredMaxSeatsResponse = {
    result: string
    message: string
}
