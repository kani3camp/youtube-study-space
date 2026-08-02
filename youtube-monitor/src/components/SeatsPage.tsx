import Image from 'next/image'
import { type FC, useMemo } from 'react'
import { calculateRoomSize } from '../lib/room-size'
import * as styles from '../styles/SeatsPage.styles'
import type { Seat } from '../types/api'
import type { RoomLayout } from '../types/room-layout'
import type { RoomViewport } from './monitor/types'
import SeatBox from './SeatBox'

export type LayoutPageProps = {
	roomLayout: RoomLayout
	usedSeats: Seat[]
	firstSeatId: number
	display: boolean
	memberOnly: boolean
	menuImageMap: Map<string, string>
	viewport: RoomViewport
}

export const SeatState = {
	Work: 'work',
	Break: 'break',
}

const SeatsPage: FC<LayoutPageProps> = ({
	roomLayout,
	usedSeats,
	firstSeatId,
	display,
	memberOnly,
	menuImageMap,
	viewport,
}) => {
	const roomSize = useMemo(
		() => calculateRoomSize(roomLayout.room_shape, viewport),
		[roomLayout.room_shape, viewport],
	)
	const scale = roomSize.width / roomLayout.room_shape.width
	const seatFontSizePx = roomSize.width * roomLayout.font_size_ratio

	const seatShape = useMemo(
		() => ({
			widthPx: roomLayout.seat_shape.width * scale,
			heightPx: roomLayout.seat_shape.height * scale,
		}),
		[roomLayout.seat_shape, scale],
	)

	const seatPositions = useMemo(
		() =>
			roomLayout.seats.map((seat) => ({
				x: (100 * seat.x) / roomLayout.room_shape.width,
				y: (100 * seat.y) / roomLayout.room_shape.height,
				rotate: seat.rotate,
			})),
		[roomLayout.seats, roomLayout.room_shape],
	)

	const partitionShapes = useMemo(
		() =>
			roomLayout.partitions.map((partition) => {
				const shape = roomLayout.partition_shapes.find(
					(partitionShape) => partitionShape.name === partition.shape_type,
				)
				return {
					widthPercent: shape
						? (100 * shape.width) / roomLayout.room_shape.width
						: 0,
					heightPercent: shape
						? (100 * shape.height) / roomLayout.room_shape.height
						: 0,
				}
			}),
		[roomLayout.partitions, roomLayout.partition_shapes, roomLayout.room_shape],
	)

	const partitionPositions = useMemo(
		() =>
			roomLayout.partitions.map((partition) => ({
				x: (100 * partition.x) / roomLayout.room_shape.width,
				y: (100 * partition.y) / roomLayout.room_shape.height,
			})),
		[roomLayout.partitions, roomLayout.room_shape],
	)

	const usedSeatIds = useMemo(
		() => new Set(usedSeats.map((seat) => seat.seat_id)),
		[usedSeats],
	)

	const seatList = useMemo(
		() =>
			roomLayout.seats.map((_seat, index) => {
				const globalSeatId = firstSeatId + index
				const isUsed = usedSeatIds.has(globalSeatId)
				const processingSeat =
					usedSeats.find((seat) => seat.seat_id === globalSeatId) ??
					usedSeats[0]
				const now = Date.now()
				const minutesElapsed = isUsed
					? Math.floor((now - processingSeat.entered_at.toMillis()) / 1000 / 60)
					: 0
				const hoursElapsed = isUsed ? Math.floor(minutesElapsed / 60) : 0
				const minutesRemaining = isUsed
					? Math.floor((processingSeat.until.toMillis() - now) / 1000 / 60)
					: 0
				const hoursRemaining = isUsed ? Math.floor(minutesRemaining / 60) : 0

				return (
					<SeatBox
						key={globalSeatId}
						globalSeatId={globalSeatId}
						isUsed={isUsed}
						memberOnly={memberOnly}
						processingSeat={processingSeat}
						seatPosition={seatPositions[index]}
						seatShape={seatShape}
						seatFontSizePx={seatFontSizePx}
						minutesElapsed={minutesElapsed}
						hoursElapsed={hoursElapsed}
						minutesRemaining={minutesRemaining}
						hoursRemaining={hoursRemaining}
						roomShape={{ widthPx: roomSize.width, heightPx: roomSize.height }}
						menuImageMap={menuImageMap}
					/>
				)
			}),
		[
			roomLayout.seats,
			usedSeats,
			firstSeatId,
			memberOnly,
			seatPositions,
			seatShape,
			seatFontSizePx,
			roomSize,
			usedSeatIds,
			menuImageMap,
		],
	)

	const partitionList = useMemo(
		() =>
			roomLayout.partitions.map((partition, index) => (
				<div
					key={partition.id}
					css={styles.partition}
					style={{
						left: `${partitionPositions[index].x}%`,
						top: `${partitionPositions[index].y}%`,
						width: `${partitionShapes[index].widthPercent}%`,
						height: `${partitionShapes[index].heightPercent}%`,
					}}
				/>
			)),
		[roomLayout.partitions, partitionPositions, partitionShapes],
	)

	return (
		<div
			css={styles.roomLayout}
			style={
				display
					? {
							display: 'block',
							width: roomSize.width,
							height: roomSize.height,
						}
					: { display: 'none' }
			}
		>
			{roomLayout.floor_image && (
				<Image
					alt="room image"
					src={roomLayout.floor_image}
					width={roomSize.width}
					height={roomSize.height}
					priority={true}
				/>
			)}

			{seatList}
			{partitionList}
		</div>
	)
}

export default SeatsPage
