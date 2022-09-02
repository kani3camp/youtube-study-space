import { css, keyframes } from '@emotion/react'
import chroma from 'chroma-js'
import { FC } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/LayoutDisplay.styles'
import { Seat } from '../types/api'
import { RoomLayout } from '../types/room-layout'

export type LayoutPageProps = {
    roomLayout: RoomLayout
    usedSeats: Seat[]
    firstSeatId: number
    display: boolean // 表示するページの場合はtrue、それ以外はfalse
}

const SeatState = {
    Work: 'work',
    Break: 'break',
}

const SeatsPage: FC<LayoutPageProps> = (props) => {
    const emptySeatColor = '#F3E8DC'

    const roomShape = {
        widthPx:
            (1000 * props.roomLayout.room_shape.width) /
            props.roomLayout.room_shape.height,
        heightPx: 1000,
    }

    const seatFontSizePx = roomShape.widthPx * props.roomLayout.font_size_ratio

    const seatShape = {
        width:
            (100 * props.roomLayout.seat_shape.width) /
            props.roomLayout.room_shape.width,
        height:
            (100 * props.roomLayout.seat_shape.height) /
            props.roomLayout.room_shape.height,
    }

    const seatPositions = props.roomLayout.seats.map((seat) => ({
        x: (100 * seat.x) / props.roomLayout.room_shape.width,
        y: (100 * seat.y) / props.roomLayout.room_shape.height,
        rotate: seat.rotate,
    }))

    const partitionShapes = props.roomLayout.partitions.map((partition) => {
        const partitionShapes = props.roomLayout.partition_shapes
        const shapeType = partition.shape_type
        let widthPercent
        let heightPercent
        for (let i = 0; i < partitionShapes.length; i++) {
            if (partitionShapes[i].name === shapeType) {
                widthPercent =
                    (100 * partitionShapes[i].width) /
                    props.roomLayout.room_shape.width
                heightPercent =
                    (100 * partitionShapes[i].height) /
                    props.roomLayout.room_shape.height
            }
        }
        return {
            widthPercent,
            heightPercent,
        }
    })

    const seatWithSeatId = (seatId: number, seats: Seat[]) => {
        let targetSeat: Seat = seats[0]
        seats.forEach((seat) => {
            if (seat.seat_id === seatId) {
                targetSeat = seat
            }
        })
        return targetSeat
    }

    const partitionPositions = props.roomLayout.partitions.map((partition) => ({
        x: (100 * partition.x) / props.roomLayout.room_shape.width,
        y: (100 * partition.y) / props.roomLayout.room_shape.height,
    }))

    const seatList = props.roomLayout.seats.map((seat, index) => {
        const usedSeatIds = props.usedSeats.map((seat) => seat.seat_id)
        const globalSeatId = props.firstSeatId + index
        const isUsed = usedSeatIds.includes(globalSeatId)
        const processingSeat = seatWithSeatId(globalSeatId, props.usedSeats)
        const workName = isUsed ? processingSeat.work_name : ''
        const breakWorkName = isUsed ? processingSeat.break_work_name : ''
        const displayName = isUsed ? processingSeat.user_display_name : ''
        const seat_color = isUsed
            ? processingSeat.appearance.color_code
            : emptySeatColor
        const isBreak = isUsed && processingSeat.state === SeatState.Break
        const glowAnimationEnabled =
            isUsed && processingSeat.appearance.glow_animation
        const numStars = isUsed ? processingSeat.appearance.num_stars : 0

        // 文字幅に応じて作業名または休憩中の作業名のフォントサイズを調整
        let workNameFontSizePx = seatFontSizePx
        if (isUsed) {
            const canvas: HTMLCanvasElement = document.createElement('canvas')
            const context = canvas.getContext('2d')
            if (context) {
                context.font = `${workNameFontSizePx.toString()}px ${
                    Constants.fontFamily
                }`
                const metrics = context.measureText(
                    isBreak ? breakWorkName : workName
                )
                const actualSeatWidth =
                    (roomShape.widthPx * seatShape.width) / 100
                if (metrics.width > actualSeatWidth) {
                    workNameFontSizePx *= actualSeatWidth / metrics.width
                    workNameFontSizePx *= 0.95 // ほんの少し縮めないと，入りきらない
                    if (workNameFontSizePx < seatFontSizePx * 0.7) {
                        workNameFontSizePx = seatFontSizePx * 0.7 // 最小でもデフォルトの0.7倍のフォントサイズ
                    }
                }
            }
        }
        const gColorLighten = chroma(seat_color).brighten(1).hex()
        const gColorDarken = chroma(seat_color).darken(2).hex()
        const glowKeyframes = keyframes`
            0% {
                background-color: ${seat_color};
            }
            50% {
                background-color: ${gColorLighten};
            }
            100% {
                background-color: ${seat_color};
            }
            `

        const glowStyle = glowAnimationEnabled
            ? css`
                  animation: ${glowKeyframes} 5s linear infinite;
                  box-shadow: inset 0 0 ${seatFontSizePx}px 0 ${gColorDarken};
              `
            : css`
                  animation: none;
                  box-shadow: none;
              `

        return (
            // 1つの座席
            <div
                key={globalSeatId}
                css={css`
                    ${styles.seat};
                    ${glowStyle};
                `}
                style={{
                    backgroundColor: seat_color,
                    left: `${seatPositions[index].x}%`,
                    top: `${seatPositions[index].y}%`,
                    transform: `rotate(${seatPositions[index].rotate}deg)`,
                    width: `${seatShape.width}%`,
                    height: `${seatShape.height}%`,
                    fontSize: isUsed
                        ? `${seatFontSizePx}px`
                        : `${seatFontSizePx * 2}px`,
                }}
            >
                {/* 席番号 */}
                <div css={styles.seatId} style={{ fontWeight: 'bold' }}>
                    {globalSeatId}
                </div>

                {/* 作業名 */}
                {(workName !== '' || breakWorkName !== '') && (
                    <div
                        css={styles.workName}
                        style={{
                            fontSize: `${workNameFontSizePx}px`,
                        }}
                    >
                        {isBreak ? breakWorkName : workName}
                    </div>
                )}

                {/* 名前 */}
                <div css={styles.userDisplayName}>{displayName}</div>

                {/* 休み中 */}
                {isBreak && (
                    <div
                        css={styles.breakBadge}
                        style={{
                            fontSize: `${seatFontSizePx * 0.6}px`,
                            borderRadius: `${seatFontSizePx / 2}px`,
                            padding: `${seatFontSizePx / 15}px`,
                            left: `${seatFontSizePx * 0.14}px`,
                            top: `${seatFontSizePx * 0.2}px`,
                            borderWidth: `${seatFontSizePx * 0.05}px`,
                        }}
                    >
                        休み
                    </div>
                )}

                {/* ★マーク */}
                {numStars > 0 && (
                    <div
                        css={styles.starsBadge}
                        style={{
                            fontSize: `${seatFontSizePx * 0.6}px`,
                            width: `${seatFontSizePx * 1.8}px`,
                            paddingTop: `${seatFontSizePx / 8}px`,
                        }}
                    >
                        {`★×${numStars}`}
                    </div>
                )}
            </div>
        )
    })

    const partitionList = props.roomLayout.partitions.map(
        (partition, index) => (
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
        )
    )

    return (
        <>
            <div
                css={styles.roomLayout}
                style={props.display ? {} : { display: 'none' }}
            >
                {props.roomLayout.floor_image && (
                    <img
                        src={props.roomLayout.floor_image}
                        width={roomShape.widthPx}
                        height={roomShape.heightPx}
                    />
                )}

                {seatList}

                {partitionList}
            </div>
        </>
    )
}

export default SeatsPage
