/** @jsxImportSource @emotion/react */
import { css, keyframes } from '@emotion/react'
import Image from 'next/image'
import type { FC } from 'react'
import { validateString } from '../lib/common'
import { Constants } from '../lib/constants'
import * as styles from '../styles/SeatBox.styles'
import type { Seat } from '../types/api'
import { SeatState } from './SeatsPage'

export type SeatProps = {
	globalSeatId: number
	isUsed: boolean
	memberOnly: boolean
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
		widthPx: number
		heightPx: number
	}
	roomShape: {
		widthPx: number
		heightPx: number
	}
	menuImageMap: Map<string, string>
}

const SeatBox: FC<SeatProps> = (props) => {
	const workName = props.isUsed ? props.processingSeat.work_name : ''
	const breakWorkName = props.isUsed ? props.processingSeat.break_work_name : ''
	const isBreak = props.isUsed && props.processingSeat.state === SeatState.Break
	const menuCode = props.isUsed ? props.processingSeat.menu_code : ''
	const numStars = props.isUsed ? props.processingSeat.appearance.num_stars : 0
	const profileImageUrl = props.isUsed
		? props.processingSeat.user_profile_image_url
		: ''
	const displayName = props.isUsed ? props.processingSeat.user_display_name : ''
	const seatColor = props.isUsed
		? props.processingSeat.appearance.color_code1
		: ''
	const colorGradientEnabled =
		props.isUsed && props.processingSeat.appearance.color_gradient_enabled

	const reloadImage = (
		e: React.SyntheticEvent<HTMLImageElement, Event>,
		imgSrc: string,
	) => {
		console.error(`retrying to load image... ' + ${imgSrc}`)
		e.currentTarget.src = `${imgSrc}?${Date.now().toString()}`
	}

	const colorGradientKeyframes = keyframes`
    0%{background-position:0% 50%}
    50%{background-position:100% 50%}
    100%{background-position:0% 50%}
`

	const topBarGradientStyle = colorGradientEnabled
		? css`
              background-image: linear-gradient(
                  90deg,
                  ${seatColor},
                  ${props.processingSeat.appearance.color_code2}
              );
              background-size: 400% 400%;
              animation: ${colorGradientKeyframes} 4s linear infinite;
          `
		: css``

	const topBarHeight = Math.max(3, props.seatFontSizePx * 0.17)
	const seatNumberFontSize = props.seatFontSizePx * 0.5
	const workNameFontSize = props.seatFontSizePx * 0.78
	const userNameFontSize = props.seatFontSizePx * 0.58
	const emptyNumberFontSize = props.seatFontSizePx * 1.3
	const emptyLabelFontSize = props.seatFontSizePx * 0.55
	const timeRemainingFontSize = props.seatFontSizePx * 0.5
	const timeElapsedFontSize = props.seatFontSizePx * 0.4

	const workContent = props.isUsed
		? !isBreak && workName
			? workName
			: isBreak && breakWorkName
				? breakWorkName
				: ''
		: ''
	const hasWorkContent = workContent !== ''

	const paddingTop = props.isUsed ? topBarHeight + seatNumberFontSize * 1.3 : 0
	const paddingBottom =
		props.isUsed && props.memberOnly ? timeRemainingFontSize * 1.6 : 0

	return (
		<div
			css={styles.seat}
			style={{
				backgroundColor: props.isUsed
					? Constants.occupiedSeatColor
					: Constants.emptySeatColor,
				left: `${props.seatPosition.x}%`,
				top: `${props.seatPosition.y}%`,
				transform: `rotate(${props.seatPosition.rotate}deg)`,
				width: `${props.seatShape.widthPx}px`,
				height: `${props.seatShape.heightPx}px`,
				fontSize: `${props.seatFontSizePx}px`,
				paddingTop: `${paddingTop}px`,
				paddingBottom: `${paddingBottom}px`,
			}}
		>
			{/* Top color bar */}
			{props.isUsed && (
				<div
					css={css`
                        ${styles.topBar};
                        ${topBarGradientStyle};
                    `}
					style={{
						height: `${topBarHeight}px`,
						backgroundColor: !colorGradientEnabled ? seatColor : undefined,
					}}
				/>
			)}

			{/* Seat number (top-left) */}
			{props.isUsed && (
				<div
					css={styles.seatNumber}
					style={{
						fontSize: `${seatNumberFontSize}px`,
						top: `${topBarHeight + 2}px`,
						left: '5%',
					}}
				>
					{props.globalSeatId}
				</div>
			)}

			{/* Break badge */}
			{isBreak && (
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

			{/* Menu icon */}
			{props.isUsed &&
				!isBreak &&
				validateString(menuCode) &&
				props.menuImageMap.get(menuCode) && (
					<Image
						alt="menu item"
						src={props.menuImageMap.get(menuCode) as string}
						css={styles.menuItem}
						width={props.seatFontSizePx * 1.55}
						height={props.seatFontSizePx * 1.55}
					/>
				)}

			{/* ★Mark */}
			{numStars > 0 && (
				<div
					css={styles.starsBadge}
					style={{
						fontSize: `${props.seatFontSizePx * 0.6}px`,
					}}
				>
					{`★×${numStars}`}
				</div>
			)}

			{/* Work name (center, largest text) */}
			{props.isUsed && hasWorkContent && (
				<div
					css={styles.workName}
					style={{ fontSize: `${workNameFontSize}px` }}
				>
					{workContent}
				</div>
			)}

			{/* User display name — general seat */}
			{props.isUsed && !props.memberOnly && (
				<div
					css={styles.userName}
					style={{ fontSize: `${userNameFontSize}px` }}
				>
					{displayName}
				</div>
			)}

			{/* User display name with small avatar — member seat, has work name */}
			{props.isUsed && props.memberOnly && hasWorkContent && (
				<div
					css={styles.userNameRow}
					style={{ fontSize: `${userNameFontSize}px` }}
				>
					<Image
						alt="profile image"
						css={styles.profileImageSmall}
						src={profileImageUrl}
						width={Constants.memberSmallIconSize}
						height={Constants.memberSmallIconSize}
						onError={(event) => reloadImage(event, profileImageUrl)}
						priority={true}
					/>
					<span css={styles.userName}>{displayName}</span>
				</div>
			)}

			{/* Large avatar + user name — member seat, no work name */}
			{props.isUsed && props.memberOnly && !hasWorkContent && (
				<>
					<Image
						alt="profile image"
						css={styles.profileImageLarge}
						src={profileImageUrl}
						width={Constants.memberBigIconSize}
						height={Constants.memberBigIconSize}
						onError={(event) => reloadImage(event, profileImageUrl)}
						priority={true}
					/>
					<div
						css={styles.userName}
						style={{ fontSize: `${userNameFontSize}px` }}
					>
						{displayName}
					</div>
				</>
			)}

			{/* Empty seat content */}
			{!props.isUsed && (
				<>
					<div
						css={styles.emptyNumber}
						style={{ fontSize: `${emptyNumberFontSize}px` }}
					>
						{props.memberOnly ? '/' : '!'}
						{props.globalSeatId}
					</div>
					<div
						css={styles.emptyLabel}
						style={{ fontSize: `${emptyLabelFontSize}px` }}
					>
						空席
					</div>
				</>
			)}

			{/* Time elapsed (sub display) — member only */}
			{props.isUsed && props.memberOnly && (
				<div
					css={styles.timeElapsed}
					style={{
						fontSize: `${timeElapsedFontSize}px`,
					}}
				>
					{props.hoursElapsed > 0
						? `${props.hoursElapsed}h ${props.minutesElapsed % 60}m`
						: `${props.minutesElapsed % 60}m`}
				</div>
			)}

			{/* Time remaining (main display) — member only */}
			{props.isUsed && props.memberOnly && (
				<div
					css={styles.timeRemaining}
					style={{
						fontSize: `${timeRemainingFontSize}px`,
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
