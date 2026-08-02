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
	ready: boolean
	generalSeats: Seat[]
	memberSeats: Seat[]
	systemConstants?: SystemConstants
}

type LatestControllerParams = UseSeatCapacityControllerParams

const useSeatCapacityController = ({
	enabled,
	ready,
	generalSeats,
	memberSeats,
	systemConstants,
}: UseSeatCapacityControllerParams): void => {
	const lastReviewKey = useRef<string>()
	const reviewInProgress = useRef(false)
	const reviewQueued = useRef(false)
	const latestParams = useRef<LatestControllerParams>({
		enabled,
		ready,
		generalSeats,
		memberSeats,
		systemConstants,
	})
	latestParams.current = {
		enabled,
		ready,
		generalSeats,
		memberSeats,
		systemConstants,
	}

	const reviewSeatCapacity = async (): Promise<void> => {
		if (reviewInProgress.current) {
			reviewQueued.current = true
			return
		}

		reviewInProgress.current = true
		try {
			do {
				reviewQueued.current = false
				const currentParams = latestParams.current
				if (
					!currentParams.enabled ||
					!currentParams.ready ||
					currentParams.systemConstants === undefined
				) {
					return
				}

				const currentSystemConstants = currentParams.systemConstants
				const reviewKey = [
					currentSystemConstants.max_seats,
					currentSystemConstants.member_max_seats,
					currentSystemConstants.min_vacancy_rate,
					currentSystemConstants.youtube_membership_enabled,
					currentSystemConstants.fixed_max_seats_enabled,
					currentParams.generalSeats.length,
					currentParams.memberSeats.length,
				].join('|')
				if (lastReviewKey.current === reviewKey) {
					continue
				}
				lastReviewKey.current = reviewKey

				const desiredGeneralMaxSeats =
					currentSystemConstants.fixed_max_seats_enabled
						? numSeatsInGeneralAllBasicRooms()
						: desiredMaxSeatsByVacancyRate(
								currentParams.generalSeats.length,
								currentSystemConstants.min_vacancy_rate,
								numSeatsInGeneralAllBasicRooms(),
								allRooms.generalTemporaryRooms,
							)
				const desiredMemberMaxSeats = calculateDesiredMemberMaxSeats(
					currentSystemConstants.youtube_membership_enabled,
					currentSystemConstants.fixed_max_seats_enabled,
					currentParams.memberSeats.length,
					currentSystemConstants.min_vacancy_rate,
					numSeatsInMemberAllBasicRooms(),
					allRooms.memberTemporaryRooms,
				)

				if (
					desiredGeneralMaxSeats === currentSystemConstants.max_seats &&
					desiredMemberMaxSeats === currentSystemConstants.member_max_seats
				) {
					continue
				}

				console.log('sending request to change max_seats')
				console.log(
					`general: ${currentSystemConstants.max_seats} => ${desiredGeneralMaxSeats}`,
				)
				console.log(
					`members-only: ${currentSystemConstants.member_max_seats} => ${desiredMemberMaxSeats}`,
				)
				await fetcher<SetDesiredMaxSeatsResponse>(api.setDesiredMaxSeats, {
					method: 'POST',
					body: JSON.stringify({
						desired_max_seats: desiredGeneralMaxSeats,
						desired_member_max_seats: desiredMemberMaxSeats,
					}),
				}).then(
					() => console.log('request succeeded'),
					(cause: unknown) => {
						console.error('request failed', cause)
					},
				)
			} while (reviewQueued.current)
		} finally {
			reviewInProgress.current = false
		}
	}

	// biome-ignore lint/correctness/useExhaustiveDependencies: reviewSeatCapacity reads latestParams so it can serialize requests while these values trigger reviews.
	useEffect(() => {
		void reviewSeatCapacity()
	}, [enabled, ready, generalSeats.length, memberSeats.length, systemConstants])
}

export default useSeatCapacityController
