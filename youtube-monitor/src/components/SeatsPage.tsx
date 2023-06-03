import { css, keyframes } from '@emotion/react'
import { FC, useMemo } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/SeatsPage.styles'
import { Seat } from '../types/api'
import { RoomLayout } from '../types/room-layout'

export type LayoutPageProps = {
    roomLayout: RoomLayout
    usedSeats: Seat[]
    firstSeatId: number
    display: boolean // 表示するページの場合はtrue、それ以外はfalse
    memberOnly: boolean
}

const SeatState = {
    Work: 'work',
    Break: 'break',
}

const SeatsPage: FC<LayoutPageProps> = (props) => {
    const propsMemo = useMemo(() => props, [props])

    const frameWidth = Constants.screenWidth - Constants.sideBarWidth
    const frameHeight = Constants.screenHeight - Constants.messageBarHeight
    const frameRatio = frameWidth / frameHeight
    const roomShapeRatio =
        propsMemo.roomLayout.room_shape.width / propsMemo.roomLayout.room_shape.height
    const roomShape =
        roomShapeRatio >= frameRatio
            ? {
                  widthPx: frameWidth,
                  heightPx: frameWidth / roomShapeRatio,
              }
            : {
                  widthPx: frameHeight * roomShapeRatio,
                  heightPx: frameHeight,
              }

    const seatFontSizePx = roomShape.widthPx * propsMemo.roomLayout.font_size_ratio

    const seatShape = {
        widthPercent:
            (100 * propsMemo.roomLayout.seat_shape.width) / propsMemo.roomLayout.room_shape.width,
        heightPercent:
            (100 * propsMemo.roomLayout.seat_shape.height) / propsMemo.roomLayout.room_shape.height,
    }

    const seatPositions = propsMemo.roomLayout.seats.map((seat) => ({
        x: (100 * seat.x) / propsMemo.roomLayout.room_shape.width,
        y: (100 * seat.y) / propsMemo.roomLayout.room_shape.height,
        rotate: seat.rotate,
    }))

    const partitionShapes = propsMemo.roomLayout.partitions.map((partition) => {
        const partitionShapes = propsMemo.roomLayout.partition_shapes
        const shapeType = partition.shape_type
        let widthPercent
        let heightPercent
        for (let i = 0; i < partitionShapes.length; i++) {
            if (partitionShapes[i].name === shapeType) {
                widthPercent =
                    (100 * partitionShapes[i].width) / propsMemo.roomLayout.room_shape.width
                heightPercent =
                    (100 * partitionShapes[i].height) / propsMemo.roomLayout.room_shape.height
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

    const partitionPositions = propsMemo.roomLayout.partitions.map((partition) => ({
        x: (100 * partition.x) / propsMemo.roomLayout.room_shape.width,
        y: (100 * partition.y) / propsMemo.roomLayout.room_shape.height,
    }))

    const usedSeatIds = propsMemo.usedSeats.map((seat) => seat.seat_id)

    const seatList = propsMemo.roomLayout.seats.map((seat, index) => {
        const globalSeatId = propsMemo.firstSeatId + index
        const isUsed = usedSeatIds.includes(globalSeatId)
        const processingSeat = seatWithSeatId(globalSeatId, propsMemo.usedSeats)
        const workName = isUsed ? processingSeat.work_name : ''
        const breakWorkName = isUsed ? processingSeat.break_work_name : ''
        const displayName = isUsed ? processingSeat.user_display_name : ''
        const seatColor = isUsed ? processingSeat.appearance.color_code1 : Constants.emptySeatColor
        const isBreak = isUsed && processingSeat.state === SeatState.Break
        const colorGradientEnabled = isUsed && processingSeat.appearance.color_gradient_enabled
        const numStars = isUsed ? processingSeat.appearance.num_stars : 0

        const profileImageUrl = isUsed ? processingSeat.user_profile_image_url : ''
        const minutesElapsed = isUsed
            ? Math.floor(
                  (new Date().valueOf() -
                      new Date(processingSeat.entered_at.toMillis()).valueOf()) /
                      1000 /
                      60
              )
            : 0
        const hoursElapsed = isUsed ? Math.floor(minutesElapsed / 60) : 0
        const minutesRemaining = isUsed
            ? Math.floor(
                  (new Date(processingSeat.until.toMillis()).valueOf() - new Date().valueOf()) /
                      1000 /
                      60
              )
            : 0
        const hoursRemaining = isUsed ? Math.floor(minutesRemaining / 60) : 0

        // 文字幅に応じて作業名または休憩中の作業名のフォントサイズを調整
        let workNameFontSizePx = seatFontSizePx
        if (isUsed && (workName !== '' || breakWorkName !== '')) {
            const canvas: HTMLCanvasElement = document.createElement('canvas')
            const context = canvas.getContext('2d')
            if (context) {
                context.font = `${workNameFontSizePx.toString()}px ${Constants.seatFontFamily}`
                const metrics = context.measureText(isBreak ? breakWorkName : workName)
                let actualSeatWidth = (roomShape.widthPx * seatShape.widthPercent) / 100
                if (props.memberOnly) {
                    actualSeatWidth =
                        (Constants.memberSeatWorkNameWidthPercent * actualSeatWidth) / 100
                }
                if (metrics.width > actualSeatWidth) {
                    workNameFontSizePx *= actualSeatWidth / metrics.width
                    workNameFontSizePx *= 0.95 // ほんの少し縮めないと，入りきらない
                    if (workNameFontSizePx < seatFontSizePx * 0.7) {
                        workNameFontSizePx = seatFontSizePx * 0.7 // 最小でもデフォルトの0.7倍のフォントサイズ
                    }
                }
            }
        }

        const colorGradientKeyframes = keyframes`
            0%{background-position:0% 50%}
            50%{background-position:100% 50%}
            100%{background-position:0% 50%}
        `

        const colorGradientStyle = colorGradientEnabled
            ? css`
                  background-image: linear-gradient(
                      90deg,
                      ${seatColor},
                      ${processingSeat.appearance.color_code2}
                  );
                  background-size: 400% 400%;
                  animation: ${colorGradientKeyframes} 4s linear infinite;
              `
            : css`
                  animation: none;
                  box-shadow: none;
              `

        let seatNo = <></>
        let userDisplayName = <></>
        if (isUsed) {
            if (props.memberOnly) {
                seatNo = <div css={styles.seatIdMember}>{globalSeatId}</div>
                userDisplayName = <div css={styles.userDisplayNameMember}>{displayName}</div>
            } else {
                seatNo = <div css={styles.seatId}>{globalSeatId}</div>
                userDisplayName = <div css={styles.userDisplayName}>{displayName}</div>
            }
        } else {
            seatNo = (
                <div css={styles.seatId} style={{ fontWeight: 'bold' }}>
                    {props.memberOnly ? '/' : ''}
                    {globalSeatId}
                </div>
            )
            userDisplayName = <div css={styles.userDisplayName}>{displayName}</div>
        }

        let workNameDisplay = <></>
        if (isUsed) {
            const content =
                !isBreak && workName ? workName : isBreak && breakWorkName ? breakWorkName : ''
            if (props.memberOnly) {
                workNameDisplay = (
                    <div
                        css={content !== '' && styles.workNameMemberBalloon}
                        style={{ fontSize: `${workNameFontSizePx}px` }}
                    >
                        <div css={content !== '' && styles.workNameMemberText}>{content}</div>
                    </div>
                )
            } else {
                workNameDisplay = (
                    <div
                        css={styles.workName}
                        style={{
                            fontSize: `${workNameFontSizePx}px`,
                        }}
                    >
                        {content}
                    </div>
                )
            }
        }

        const reloadImage = (e: React.SyntheticEvent<HTMLImageElement, Event>, imgSrc: string) => {
            console.error(`retrying to load image... ' + ${imgSrc}`)
            e.currentTarget.src = `${imgSrc}?${new Date().getTime().toString()}`
        }

        return (
            // for each seat
            <div
                key={globalSeatId}
                css={css`
                    ${styles.seat};
                    ${colorGradientStyle};
                `}
                style={{
                    backgroundColor: seatColor,
                    left: `${seatPositions[index].x}%`,
                    top: `${seatPositions[index].y}%`,
                    transform: `rotate(${seatPositions[index].rotate}deg)`,
                    width: `${seatShape.widthPercent}%`,
                    height: `${seatShape.heightPercent}%`,
                    fontSize: isUsed ? `${seatFontSizePx}px` : `${seatFontSizePx * 2}px`,
                }}
            >
                {/* seat No. */}
                {seatNo}

                {/* work name */}
                {workNameDisplay}

                {/* display name */}
                {userDisplayName}

                {/* break mode */}
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

                {/* ★Mark */}
                {numStars > 0 && (
                    <div
                        css={styles.starsBadge}
                        style={{
                            fontSize: `${seatFontSizePx * 0.6}px`,
                        }}
                    >
                        {`★×${numStars}`}
                    </div>
                )}

                {/* profile image */}
                {isUsed && props.memberOnly && (
                    <img
                        css={
                            (isBreak ? breakWorkName : workName !== '')
                                ? styles.profileImageMemberWithWorkName
                                : styles.profileImageMemberNoWorkName
                        }
                        src={profileImageUrl}
                        onError={(event) => reloadImage(event, profileImageUrl)}
                    />
                )}

                {/* time elapsed */}
                {isUsed && props.memberOnly && (
                    <div
                        css={styles.timeElapsed}
                        style={{
                            fontSize: `${seatFontSizePx * 0.6}px`,
                        }}
                    >
                        {hoursElapsed > 0
                            ? `${hoursElapsed}h ${minutesElapsed % 60}m`
                            : `${minutesElapsed % 60}m`}
                    </div>
                )}

                {/* time remaining */}
                {isUsed && props.memberOnly && (
                    <div
                        css={styles.timeRemaining}
                        style={{
                            fontSize: `${seatFontSizePx * 0.6}px`,
                        }}
                    >
                        あと
                        {hoursRemaining > 0
                            ? `${hoursRemaining}h ${minutesRemaining % 60}m`
                            : `${minutesRemaining}m`}
                    </div>
                )}
            </div>
        )
    })

    const partitionList = propsMemo.roomLayout.partitions.map((partition, index) => (
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
    ))

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
                    <img
                        src={propsMemo.roomLayout.floor_image}
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
