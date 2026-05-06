import type { RoomLayout } from '../types/room-layout'

const numSeatsOfRoomLayouts = (layouts: RoomLayout[]): number =>
	layouts.reduce((count, layout) => count + layout.seats.length, 0)

export const expandSeatsWithTemporaryRooms = (
	minSeats: number,
	basicRoomSeats: number,
	temporaryRooms: RoomLayout[],
): number => {
	let currentSeats = basicRoomSeats
	let currentTemporaryRoomIndex = 0
	while (currentSeats < minSeats) {
		currentSeats += temporaryRooms[currentTemporaryRoomIndex].seats.length
		currentTemporaryRoomIndex =
			(currentTemporaryRoomIndex + 1) % temporaryRooms.length
	}
	return currentSeats
}

export const desiredMaxSeatsByVacancyRate = (
	numUsedSeats: number,
	minVacancyRate: number,
	basicRoomSeats: number,
	temporaryRooms: RoomLayout[],
): number => {
	if (!Number.isFinite(minVacancyRate) || minVacancyRate >= 1) {
		return basicRoomSeats
	}

	const normalizedMinVacancyRate = Math.max(minVacancyRate, 0)
	const minSeatsByVacancyRate = Math.ceil(
		numUsedSeats / (1 - normalizedMinVacancyRate),
	)
	return expandSeatsWithTemporaryRooms(
		minSeatsByVacancyRate,
		basicRoomSeats,
		temporaryRooms,
	)
}

export const buildRoomLayouts = (
	basicRooms: RoomLayout[],
	temporaryRooms: RoomLayout[],
	maxSeats: number,
	fixedMaxSeatsEnabled: boolean,
): RoomLayout[] => {
	const nextLayouts = [...basicRooms]
	let currentSeats = numSeatsOfRoomLayouts(nextLayouts)
	if (fixedMaxSeatsEnabled || maxSeats <= currentSeats) {
		return nextLayouts
	}

	let currentTemporaryRoomIndex = 0
	while (currentSeats < maxSeats) {
		const temporaryRoom = temporaryRooms[currentTemporaryRoomIndex]
		nextLayouts.push(temporaryRoom)
		currentSeats += temporaryRoom.seats.length
		currentTemporaryRoomIndex =
			(currentTemporaryRoomIndex + 1) % temporaryRooms.length
	}
	return nextLayouts
}

export const desiredMemberMaxSeats = (
	membershipEnabled: boolean,
	fixedMaxSeatsEnabled: boolean,
	numUsedSeats: number,
	minVacancyRate: number,
	basicRoomSeats: number,
	temporaryRooms: RoomLayout[],
): number => {
	if (!membershipEnabled) {
		return 0
	}
	if (fixedMaxSeatsEnabled) {
		return basicRoomSeats
	}
	return desiredMaxSeatsByVacancyRate(
		numUsedSeats,
		minVacancyRate,
		basicRoomSeats,
		temporaryRooms,
	)
}
