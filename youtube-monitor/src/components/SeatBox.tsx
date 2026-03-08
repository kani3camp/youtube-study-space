/** @jsxImportSource @emotion/react */
import { css, keyframes } from '@emotion/react'
import Image from 'next/image'
import type { FC, SyntheticEvent } from 'react'
import { fontFamily, validateString } from '../lib/common'
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

const colorGradientKeyframes = keyframes`
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
`

const SeatBox: FC<SeatProps> = (props) => {
	const workName = props.isUsed ? props.processingSeat.work_name : ''
	const breakWorkName = props.isUsed ? props.processingSeat.break_work_name : ''
	const isBreak = props.isUsed && props.processingSeat.state === SeatState.Break
	const displayName = props.isUsed ? props.processingSeat.user_display_name : ''
	const menuCode = props.isUsed ? props.processingSeat.menu_code : ''
	const numStars = props.isUsed ? props.processingSeat.appearance.num_stars : 0
	const profileImageUrl = props.isUsed
		? props.processingSeat.user_profile_image_url
		: ''
	const currentWorkName =
		isBreak && validateString(breakWorkName) ? breakWorkName : workName
	const hasMemberWorkName = props.memberOnly && validateString(currentWorkName)
	const menuImageSrc =
		props.isUsed && !isBreak && validateString(menuCode)
			? props.menuImageMap.get(menuCode)
			: undefined
	const hasProfileImage =
		props.isUsed && props.memberOnly && validateString(profileImageUrl)
	const profileImageSize = hasMemberWorkName
		? Constants.memberSmallIconSize
		: Constants.memberBigIconSize

	const reloadImage = (
		event: SyntheticEvent<HTMLImageElement, Event>,
		imageSrc: string,
	) => {
		console.error(`retrying to load image... ${imageSrc}`)
		event.currentTarget.src = `${imageSrc}?${Date.now().toString()}`
	}

	const remainingLabel =
		props.hoursRemaining > 0
			? `あと${props.hoursRemaining}h ${props.minutesRemaining % 60}m`
			: `あと${Math.max(props.minutesRemaining, 0)}m`
	const elapsedLabel =
		props.hoursElapsed > 0
			? `${props.hoursElapsed}h ${props.minutesElapsed % 60}m`
			: `${Math.max(props.minutesElapsed, 0)}m`

	const seatIdLabel = props.isUsed
		? `${props.globalSeatId}`
		: props.memberOnly
			? `/${props.globalSeatId}`
			: `!${props.globalSeatId}`

	// 文字幅に応じて作業名または休憩中の作業名のフォントサイズを調整
	let generalWorkNameFontSizePx = props.seatFontSizePx * 0.8
	if (props.isUsed && !props.memberOnly && currentWorkName !== '') {
		const canvas: HTMLCanvasElement = document.createElement('canvas')
		const context = canvas.getContext('2d')
		if (context) {
			context.font = `${generalWorkNameFontSizePx.toString()}px ${fontFamily}`
			const metrics = context.measureText(currentWorkName)
			const actualSeatWidth = props.seatShape.widthPx

			if (metrics.width > actualSeatWidth) {
				generalWorkNameFontSizePx *= actualSeatWidth / metrics.width
				generalWorkNameFontSizePx *= 0.95 // ほんの少し縮めないと，入りきらない
				if (generalWorkNameFontSizePx < props.seatFontSizePx * 0.5) {
					generalWorkNameFontSizePx = props.seatFontSizePx * 0.5 // 最小でもデフォルトの0.7倍のフォントサイズ
				}
			}
		}
	}

	return (
		<div
			key={props.globalSeatId}
			css={styles.seat}
			style={{
				left: `${props.seatPosition.x}%`,
				top: `${props.seatPosition.y}%`,
				transform: `rotate(${props.seatPosition.rotate}deg)`,
				width: `${props.seatShape.widthPx}px`,
				height: `${props.seatShape.heightPx}px`,
				fontSize: `${props.seatFontSizePx}px`,
			}}
		>
			{/* Accent Bar */}
			{props.isUsed && (
				<div
					css={css`
                    ${styles.accentBar};
                    ${
											props.isUsed
												? props.processingSeat.appearance.color_gradient_enabled
													? css`
                    background-image: linear-gradient(
                        90deg,
                        ${props.processingSeat.appearance.color_code1},
                        ${props.processingSeat.appearance.color_code2}
                    );
                    background-size: 300% 300%;
                    animation: ${colorGradientKeyframes} 4s linear infinite;
                    mask-image: linear-gradient(
                        rgba(0, 0, 0, 1) 0%,
                        rgba(0, 0, 0, 0.75) 35%,
                        rgba(0, 0, 0, 0) 100%
                    );
                `
													: css`
                    background-color: ${props.processingSeat.appearance.color_code1};
                    mask-image: linear-gradient(
                        rgba(0, 0, 0, 1) 0%,
						rgba(0, 0, 0, 0.5)30%,
                        rgba(0, 0, 0, 0) 100%
                    );
                `
												: css`
               	background-color: rgba(0, 0, 0, 0);
            `
										};
                `}
					style={{
						height: `${Math.max(
							18,
							Math.round(props.seatShape.heightPx / 5),
						)}px`,
					}}
				/>
			)}

			{/* ★Mark */}
			{numStars > 0 && (
				<div
					css={styles.starsBadge}
					style={{
						fontSize: `${props.seatFontSizePx * 0.45}px`,
					}}
				>
					{`★×${numStars}`}
				</div>
			)}

			<div css={styles.seatBody}>
				{props.isUsed && (
					<div css={styles.headerRow}>
						<div css={styles.headerLeft}>
							<div
								css={
									props.memberOnly ? styles.memberSeatId : styles.generalSeatId
								}
							>
								{seatIdLabel}
							</div>

							{isBreak && (
								<div
									css={styles.breakBadge}
									style={{
										fontSize: `${props.seatFontSizePx * 0.45}px`,
										borderRadius: `${props.seatFontSizePx / 2}px`,
										padding: `${props.seatFontSizePx * 0.08}px ${props.seatFontSizePx * 0.18}px`,
									}}
								>
									休み
								</div>
							)}

							{menuImageSrc && !isBreak && (
								<Image
									alt="menu item"
									src={menuImageSrc}
									css={styles.menuItem}
									width={props.seatFontSizePx * 0.78}
									height={props.seatFontSizePx * 0.78}
								/>
							)}
						</div>
					</div>
				)}

				{props.isUsed ? (
					props.memberOnly ? (
						<>
							<div css={styles.memberContent}>
								<div css={styles.memberMain}>
									{/* work name */}
									{currentWorkName !== '' && (
										<div css={styles.memberWorkNameFrame}>
											<div
												css={styles.memberWorkName}
												style={{ fontSize: `${props.seatFontSizePx * 0.78}px` }}
											>
												{currentWorkName}
											</div>
										</div>
									)}

									{/* identity */}
									<div css={styles.memberIdentityRow}>
										{hasProfileImage && (
											<Image
												alt="profile image"
												css={styles.profileImage}
												src={profileImageUrl}
												width={profileImageSize}
												height={profileImageSize}
												onError={(event) => reloadImage(event, profileImageUrl)}
												priority={true}
											/>
										)}
										<div css={styles.memberDisplayName}>{displayName}</div>
									</div>
								</div>
							</div>

							{/* time elapsed */}
							<div css={styles.timeElapsed}>{elapsedLabel}</div>

							{/* time remaining */}
							<div css={styles.timeRemaining}>{remainingLabel}</div>
						</>
					) : (
						<div css={styles.generalContent}>
							{currentWorkName !== '' && (
								<div
									css={styles.generalWorkName}
									style={{ fontSize: `${generalWorkNameFontSizePx}px` }}
								>
									{currentWorkName}
								</div>
							)}
							<div css={styles.generalDisplayName}>{displayName}</div>
						</div>
					)
				) : (
					<div css={styles.emptyContent}>
						<div
							css={styles.emptySeatCommand}
							style={{
								fontSize: `${props.seatFontSizePx * (props.memberOnly ? 1.45 : 1.35)}px`,
							}}
						>
							{seatIdLabel}
						</div>
					</div>
				)}
			</div>
		</div>
	)
}

export default SeatBox
