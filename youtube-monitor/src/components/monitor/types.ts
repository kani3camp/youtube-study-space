import type { Seat } from '../../types/api'
import type { RoomLayout } from '../../types/room-layout'

export type MonitorVariant = 'horizontal' | 'vertical'

export type RoomViewport = {
	width: number
	height: number
}

export type RoomPage = {
	roomLayout: RoomLayout
	usedSeats: Seat[]
	firstSeatId: number
	memberOnly: boolean
}
