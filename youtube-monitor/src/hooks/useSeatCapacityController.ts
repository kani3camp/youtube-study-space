import { useEffect, useRef } from 'react'
import api from '../lib/api-config'
import fetcher from '../lib/fetcher'
import type { SystemConstants } from '../lib/firestore'
import {
	desiredMemberMaxSeats as calculateDesiredMemberMaxSeats,
	desiredMaxSeatsByVacancyRate,
} from '../rooms/max-seats'
import {
	allRooms,
	numSeatsInGeneralAllBasicRooms,
	numSeatsInMemberAllBasicRooms,
} from '../rooms/rooms-config'
import type { Seat, SetDesiredMaxSeatsResponse } from '../types/api'

type UseSeatCapacityControllerParams = {
	enabled: boolean
	generalSeats: Seat[]
	memberSeats: Seat[]
	systemConstants?: SystemConstants
}

const useSeatCapacityController = ({
	enabled,
	generalSeats,
	memberSeats,
	systemConstants,
}: UseSeatCapacityControllerParams): void => {
	const lastReviewKey = useRef<string>()

	useEffect(() => {
		if (!enabled || systemConstants === undefined) {
			return
		}

		const reviewKey = [
			systemConstants.max_seats,
			systemConstants.member_max_seats,
			systemConstants.min_vacancy_rate,
			systemConstants.youtube_membership_enabled,
			systemConstants.fixed_max_seats_enabled,
			generalSeats.length,
			memberSeats.length,
		].join('|')
		if (lastReviewKey.current === reviewKey) {
			return
		}
		lastReviewKey.current = reviewKey

		const desiredGeneralMaxSeats = systemConstants.fixed_max_seats_enabled
			? numSeatsInGeneralAllBasicRooms()
			: desiredMaxSeatsByVacancyRate(
					generalSeats.length,
					systemConstants.min_vacancy_rate,
					numSeatsInGeneralAllBasicRooms(),
					allRooms.generalTemporaryRooms,
				)
		const desiredMemberMaxSeats = calculateDesiredMemberMaxSeats(
			systemConstants.youtube_membership_enabled,
			systemConstants.fixed_max_seats_enabled,
			memberSeats.length,
			systemConstants.min_vacancy_rate,
			numSeatsInMemberAllBasicRooms(),
			allRooms.memberTemporaryRooms,
		)

		if (
			desiredGeneralMaxSeats === systemConstants.max_seats &&
			desiredMemberMaxSeats === systemConstants.member_max_seats
		) {
			return
		}

		console.log('sending request to change max_seats')
		console.log(
			`general: ${systemConstants.max_seats} => ${desiredGeneralMaxSeats}`,
		)
		console.log(
			`members-only: ${systemConstants.member_max_seats} => ${desiredMemberMaxSeats}`,
		)
		void fetcher<SetDesiredMaxSeatsResponse>(api.setDesiredMaxSeats, {
			method: 'POST',
			body: JSON.stringify({
				desired_max_seats: desiredGeneralMaxSeats,
				desired_member_max_seats: desiredMemberMaxSeats,
			}),
		}).then(
			() => console.log('request succeeded'),
			(cause: unknown) => console.error('request failed', cause),
		)
	}, [enabled, generalSeats.length, memberSeats.length, systemConstants])
}

export default useSeatCapacityController
