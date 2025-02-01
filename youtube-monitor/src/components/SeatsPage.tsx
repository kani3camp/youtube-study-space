import Image from 'next/image'
import { type FC, useMemo } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/SeatsPage.styles'
import type { Seat } from '../types/api'
import type { RoomLayout } from '../types/room-layout'
import SeatBox from './SeatBox'

export type LayoutPageProps = {
	roomLayout: RoomLayout
	usedSeats: Seat[]
	firstSeatId: number
	display: boolean // 表示するページの場合はtrue、それ以外はfalse
	memberOnly: boolean
}

export const SeatState = {
	Work: 'work',
	Break: 'break',
}

const SeatsPage: FC<LayoutPageProps> = (props) => {
	const propsMemo = useMemo(() => props, [props])

	const roomShape = useMemo(() => {
		const frameWidth = Constants.screenWidth - Constants.sideBarWidth
		const frameHeight = Constants.screenHeight - Constants.messageBarHeight
		const frameRatio = frameWidth / frameHeight
		const roomShapeRatio =
			propsMemo.roomLayout.room_shape.width /
			propsMemo.roomLayout.room_shape.height

		return roomShapeRatio >= frameRatio
			? {
					widthPx: frameWidth,
					heightPx: frameWidth / roomShapeRatio,
				}
			: {
					widthPx: frameHeight * roomShapeRatio,
					heightPx: frameHeight,
				}
	}, [propsMemo.roomLayout.room_shape])

	const scale = roomShape.widthPx / propsMemo.roomLayout.room_shape.width

	const seatFontSizePx =
		roomShape.widthPx * propsMemo.roomLayout.font_size_ratio

	const seatShape = useMemo(
		() => ({
			widthPx: propsMemo.roomLayout.seat_shape.width * scale,
			heightPx: propsMemo.roomLayout.seat_shape.height * scale,
		}),
		[propsMemo.roomLayout.seat_shape, scale],
	)

	const seatPositions = useMemo(
		() =>
			propsMemo.roomLayout.seats.map((seat) => ({
				x: (100 * seat.x) / propsMemo.roomLayout.room_shape.width,
				y: (100 * seat.y) / propsMemo.roomLayout.room_shape.height,
				rotate: seat.rotate,
			})),
		[propsMemo.roomLayout.seats, propsMemo.roomLayout.room_shape],
	)

	const partitionShapes = useMemo(
		() =>
			propsMemo.roomLayout.partitions.map((partition) => {
				const partitionShapes = propsMemo.roomLayout.partition_shapes
				const shapeType = partition.shape_type
				let widthPercent = 0
				let heightPercent = 0
				for (let i = 0; i < partitionShapes.length; i++) {
					if (partitionShapes[i].name === shapeType) {
						widthPercent =
							(100 * partitionShapes[i].width) /
							propsMemo.roomLayout.room_shape.width
						heightPercent =
							(100 * partitionShapes[i].height) /
							propsMemo.roomLayout.room_shape.height
					}
				}
				return {
					widthPercent,
					heightPercent,
				}
			}),
		[
			propsMemo.roomLayout.partitions,
			propsMemo.roomLayout.partition_shapes,
			propsMemo.roomLayout.room_shape,
		],
	)

	const seatWithSeatId = useMemo(
		() => (seatId: number, seats: Seat[]) => {
			return seats.find((seat) => seat.seat_id === seatId) ?? seats[0]
		},
		[],
	)

	const partitionPositions = propsMemo.roomLayout.partitions.map(
		(partition) => ({
			x: (100 * partition.x) / propsMemo.roomLayout.room_shape.width,
			y: (100 * partition.y) / propsMemo.roomLayout.room_shape.height,
		}),
	)

	const usedSeatIds = propsMemo.usedSeats.map((seat) => seat.seat_id)

	const seatList = useMemo(
		() =>
			propsMemo.roomLayout.seats.map((seat, index) => {
				const globalSeatId = propsMemo.firstSeatId + index
				const isUsed = usedSeatIds.includes(globalSeatId)
				const processingSeat = seatWithSeatId(globalSeatId, propsMemo.usedSeats)

				const minutesElapsed = isUsed
					? Math.floor(
							(new Date().valueOf() -
								new Date(processingSeat.entered_at.toMillis()).valueOf()) /
								1000 /
								60,
						)
					: 0
				const hoursElapsed = isUsed ? Math.floor(minutesElapsed / 60) : 0
				const minutesRemaining = isUsed
					? Math.floor(
							(new Date(processingSeat.until.toMillis()).valueOf() -
								new Date().valueOf()) /
								1000 /
								60,
						)
					: 0
				const hoursRemaining = isUsed ? Math.floor(minutesRemaining / 60) : 0

				return (
					<SeatBox
						key={globalSeatId}
						globalSeatId={globalSeatId}
						isUsed={isUsed}
						memberOnly={propsMemo.memberOnly}
						processingSeat={processingSeat}
						seatPosition={seatPositions[index]}
						seatShape={seatShape}
						seatFontSizePx={seatFontSizePx}
						minutesElapsed={minutesElapsed}
						hoursElapsed={hoursElapsed}
						minutesRemaining={minutesRemaining}
						hoursRemaining={hoursRemaining}
						roomShape={roomShape}
					/>
				)
			}),
		[
			propsMemo.roomLayout.seats,
			propsMemo.usedSeats,
			propsMemo.firstSeatId,
			seatPositions,
			roomShape,
			propsMemo.memberOnly,
			seatShape,
			usedSeatIds,
			seatFontSizePx,
			seatWithSeatId,
		],
	)

	const partitionList = useMemo(
		() =>
			propsMemo.roomLayout.partitions.map((partition, index) => (
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
		[propsMemo.roomLayout.partitions, partitionPositions, partitionShapes],
	)

	return (
		<>
			<div
				css={styles.roomLayout}
				style={
					propsMemo.display
						? {
								display: 'block',
								width: roomShape.widthPx,
								height: roomShape.heightPx,
							}
						: {
								display: 'none',
							}
				}
			>
				{propsMemo.roomLayout.floor_image && (
					<Image
						alt="room image"
						src={propsMemo.roomLayout.floor_image}
						width={roomShape.widthPx}
						height={roomShape.heightPx}
						priority={true}
					/>
				)}

				{seatList}

				{partitionList}
			</div>
		</>
	)
}

export default SeatsPage
