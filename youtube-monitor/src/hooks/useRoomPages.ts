import { useMemo } from 'react'
import type { RoomPage } from '../components/monitor/types'
import type { SystemConstants } from '../lib/firestore'
import { buildRoomLayouts } from '../rooms/max-seats'
import { allRooms } from '../rooms/rooms-config'
import type { Seat } from '../types/api'

type UseRoomPagesParams = {
	generalSeats: Seat[]
	memberSeats: Seat[]
	systemConstants?: SystemConstants
}

const buildPagesForLayouts = (
	layouts: ReturnType<typeof buildRoomLayouts>,
	seats: Seat[],
	memberOnly: boolean,
): RoomPage[] => {
	let nextFirstSeatId = 1
	return layouts.map((roomLayout) => {
		const firstSeatId = nextFirstSeatId
		nextFirstSeatId += roomLayout.seats.length
		const lastSeatId = nextFirstSeatId - 1

		return {
			roomLayout,
			firstSeatId,
			usedSeats: seats.filter(
				(seat) => seat.seat_id >= firstSeatId && seat.seat_id <= lastSeatId,
			),
			memberOnly,
		}
	})
}

export const buildRoomPages = ({
	generalSeats,
	memberSeats,
	systemConstants,
}: UseRoomPagesParams): RoomPage[] => {
	if (systemConstants === undefined) {
		return []
	}

	const generalPages = buildPagesForLayouts(
		buildRoomLayouts(
			allRooms.generalBasicRooms,
			allRooms.generalTemporaryRooms,
			systemConstants.max_seats,
			systemConstants.fixed_max_seats_enabled,
		),
		generalSeats,
		false,
	)
	if (!systemConstants.youtube_membership_enabled) {
		return generalPages
	}

	return generalPages.concat(
		buildPagesForLayouts(
			buildRoomLayouts(
				allRooms.memberBasicRooms,
				allRooms.memberTemporaryRooms,
				systemConstants.member_max_seats,
				systemConstants.fixed_max_seats_enabled,
			),
			memberSeats,
			true,
		),
	)
}

const useRoomPages = (params: UseRoomPagesParams): RoomPage[] => {
	const { generalSeats, memberSeats, systemConstants } = params
	return useMemo(
		() => buildRoomPages({ generalSeats, memberSeats, systemConstants }),
		[generalSeats, memberSeats, systemConstants],
	)
}

export default useRoomPages
