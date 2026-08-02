import type { CSSProperties } from 'react'
import type { Menu, Seat } from '../../types/api'
import type { RoomLayout } from '../../types/room-layout'

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

export type PositionedComponentStyle = CSSProperties

export type MonitorMenuData = {
	menuItems: Menu[]
	menuImageMap: Map<string, string>
}
