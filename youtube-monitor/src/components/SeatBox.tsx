import { FC } from 'react'
import * as styles from '../styles/SeatsPage.styles'
import { css, keyframes } from '@emotion/react'
import { Seat } from '../types/api'
import { Constants } from '../lib/constants'

type SeatProps = {
    globalSeatId: number
    workName: string
    breakWorkName: string
    isBreak: boolean
    numStars: number
    isUsed: boolean
    memberOnly: boolean
    seatColor: string
    profileImageUrl: string
    hoursRemaining: number
    minutesRemaining: number
    hoursElapsed: number
    minutesElapsed: number
    seatFontSizePx: number
    processingSeat: Seat
    seatPosition: {
        x: number
        y: number
        rotate: number
    }
    seatShape: {
        widthPercent: number
        heightPercent: number
    }
    roomShape: {
        widthPx: number
        heightPx: number
    }
}

const SeatBox: FC<SeatProps> = (props) => {
    const colorGradientEnabled =
        props.isUsed && props.processingSeat.appearance.color_gradient_enabled

    const reloadImage = (e: React.SyntheticEvent<HTMLImageElement, Event>, imgSrc: string) => {
        console.error(`retrying to load image... ' + ${imgSrc}`)
        e.currentTarget.src = `${imgSrc}?${new Date().getTime().toString()}`
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
                  ${props.seatColor},
                  ${props.processingSeat.appearance.color_code2}
              );
              background-size: 400% 400%;
              animation: ${colorGradientKeyframes} 4s linear infinite;
          `
        : css`
              animation: none;
              box-shadow: none;
          `

    const displayName = props.isUsed ? props.processingSeat.user_display_name : ''

    let seatNo = <></>
    let userDisplayName = <></>
    if (props.isUsed) {
        if (props.memberOnly) {
            seatNo = <div css={styles.seatIdMember}>{props.globalSeatId}</div>
            userDisplayName = <div css={styles.userDisplayNameMember}>{displayName}</div>
        } else {
            seatNo = <div css={styles.seatId}>{props.globalSeatId}</div>
            userDisplayName = <div css={styles.userDisplayName}>{displayName}</div>
        }
    } else {
        seatNo = (
            <div css={styles.seatId} style={{ fontWeight: 'bold' }}>
                {props.memberOnly ? '/' : ''}
                {props.globalSeatId}
            </div>
        )
        userDisplayName = <div css={styles.userDisplayName}>{displayName}</div>
    }

    // 文字幅に応じて作業名または休憩中の作業名のフォントサイズを調整
    let workNameFontSizePx = props.seatFontSizePx
    if (props.isUsed && (props.workName !== '' || props.breakWorkName !== '')) {
        const canvas: HTMLCanvasElement = document.createElement('canvas')
        const context = canvas.getContext('2d')
        if (context) {
            context.font = `${workNameFontSizePx.toString()}px ${Constants.seatFontFamily}`
            const metrics = context.measureText(
                props.isBreak ? props.breakWorkName : props.workName
            )
            let actualSeatWidth = (props.roomShape.widthPx * props.seatShape.widthPercent) / 100
            if (props.memberOnly) {
                actualSeatWidth = (Constants.memberSeatWorkNameWidthPercent * actualSeatWidth) / 100
            }
            if (metrics.width > actualSeatWidth) {
                workNameFontSizePx *= actualSeatWidth / metrics.width
                workNameFontSizePx *= 0.95 // ほんの少し縮めないと，入りきらない
                if (workNameFontSizePx < props.seatFontSizePx * 0.7) {
                    workNameFontSizePx = props.seatFontSizePx * 0.7 // 最小でもデフォルトの0.7倍のフォントサイズ
                }
            }
        }
    }

    let workNameDisplay = <></>
    if (props.isUsed) {
        const content =
            !props.isBreak && props.workName
                ? props.workName
                : props.isBreak && props.breakWorkName
                ? props.breakWorkName
                : ''
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

    return (
        <div
            key={props.globalSeatId}
            css={css`
                ${styles.seat};
                ${colorGradientStyle};
            `}
            style={{
                backgroundColor: props.seatColor,
                left: `${props.seatPosition.x}%`,
                top: `${props.seatPosition.y}%`,
                transform: `rotate(${props.seatPosition.rotate}deg)`,
                width: `${props.seatShape.widthPercent}%`,
                height: `${props.seatShape.heightPercent}%`,
                fontSize: props.isUsed
                    ? `${props.seatFontSizePx}px`
                    : `${props.seatFontSizePx * 2}px`,
            }}
        >
            {/* seat No. */}
            {seatNo}

            {/* work name */}
            {workNameDisplay}

            {/* display name */}
            {userDisplayName}

            {/* break mode */}
            {props.isBreak && (
                <div
                    css={styles.breakBadge}
                    style={{
                        fontSize: `${props.seatFontSizePx * 0.6}px`,
                        borderRadius: `${props.seatFontSizePx / 2}px`,
                        padding: `${props.seatFontSizePx / 15}px`,
                        left: `${props.seatFontSizePx * 0.14}px`,
                        top: `${props.seatFontSizePx * 0.2}px`,
                        borderWidth: `${props.seatFontSizePx * 0.05}px`,
                    }}
                >
                    休み
                </div>
            )}

            {/* ★Mark */}
            {props.numStars > 0 && (
                <div
                    css={styles.starsBadge}
                    style={{
                        fontSize: `${props.seatFontSizePx * 0.6}px`,
                    }}
                >
                    {`★×${props.numStars}`}
                </div>
            )}

            {/* profile image */}
            {props.isUsed && props.memberOnly && (
                <img
                    css={
                        (props.isBreak ? props.breakWorkName : props.workName !== '')
                            ? styles.profileImageMemberWithWorkName
                            : styles.profileImageMemberNoWorkName
                    }
                    src={props.profileImageUrl}
                    onError={(event) => reloadImage(event, props.profileImageUrl)}
                />
            )}

            {/* time elapsed */}
            {props.isUsed && props.memberOnly && (
                <div
                    css={styles.timeElapsed}
                    style={{
                        fontSize: `${props.seatFontSizePx * 0.6}px`,
                    }}
                >
                    {props.hoursElapsed > 0
                        ? `${props.hoursElapsed}h ${props.minutesElapsed % 60}m`
                        : `${props.minutesElapsed % 60}m`}
                </div>
            )}

            {/* time remaining */}
            {props.isUsed && props.memberOnly && (
                <div
                    css={styles.timeRemaining}
                    style={{
                        fontSize: `${props.seatFontSizePx * 0.6}px`,
                    }}
                >
                    あと
                    {props.hoursRemaining > 0
                        ? `${props.hoursRemaining}h ${props.minutesRemaining % 60}m`
                        : `${props.minutesRemaining}m`}
                </div>
            )}
        </div>
    )
}

export default SeatBox
