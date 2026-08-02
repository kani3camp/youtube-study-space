import { validateString } from './common'

if (
	process.env.NEXT_PUBLIC_DEBUG !== 'true' &&
	process.env.NEXT_PUBLIC_DEBUG !== 'false'
) {
	throw Error(
		`invalid NEXT_PUBLIC_DEBUG: ${process.env.NEXT_PUBLIC_DEBUG?.toString()}`,
	)
}
if (
	process.env.NEXT_PUBLIC_CHANNEL_GL !== 'true' &&
	process.env.NEXT_PUBLIC_CHANNEL_GL !== 'false'
) {
	throw Error(
		`invalid NEXT_PUBLIC_CHANNEL_GL: ${process.env.NEXT_PUBLIC_CHANNEL_GL?.toString()}`,
	)
}
export const DEBUG = process.env.NEXT_PUBLIC_DEBUG === 'true'

if (!validateString(process.env.NEXT_PUBLIC_ROOM_CONFIG)) {
	throw Error(
		`invalid NEXT_PUBLIC_ROOMS_CONFIG: ${process.env.NEXT_PUBLIC_ROOM_CONFIG?.toString()}`,
	)
}
export const ROOM_CONFIG = process.env.NEXT_PUBLIC_ROOM_CONFIG

export const sidebarCardVerticalInsetPx = 16
export const sidebarCardHorizontalInsetPx = 40

export const Constants = {
	screenWidth: 1920,
	screenHeight: 1080,
	sideBarWidth: 400,
	tickerWidth: 600,
	messageBarHeight: 80,
	clockHeight: 105,
	usageHeight: 200,
	colorBarHeight: 100,
	menuHeight: 310,
	timerHeight: 260,
	seatBackgroundColor: '#F4EFE7',
	vacantSeatBackgroundColor: '#ded5c6ff',
	breakBadgeZIndex: 10,
	bgmVolume: DEBUG ? 0.1 : 0.3,
	chimeVolume: 0.7,
	chimeSingleFilePath: '/chime/chime1.mp3',
	chimeDoubleFilePath: '/chime/chime2.mp3',
	pagingIntervalSeconds: 8,
	timerProgressStudyColor: '#e03c00',
	timerProgressBreakColor: '#008c36',
	primaryTextColor: '#3a1e86',
	secondaryTextColor: '#f1e8f2',
	memberBigIconSize: 44,
	memberSmallIconSize: 26,
	menuIconSize: 80,
}

export const sidebarBgmHeight =
	Constants.screenHeight -
	Constants.clockHeight -
	Constants.usageHeight -
	Constants.menuHeight -
	Constants.timerHeight -
	Constants.colorBarHeight

export const debug = false
