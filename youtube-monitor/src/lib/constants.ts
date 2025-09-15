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

export const Constants = {
	screenWidth: 1920,
	screenHeight: 1080,
	sideBarWidth: 400,
	tickerWidth: 600,
	messageBarHeight: 80,
	clockHeight: 160,
	usageHeight: 315,
	menuHeight: 310,
	timerHeight: 210,
	breakBadgeZIndex: 10,
	bgmVolume: DEBUG ? 0.1 : 0.3,
	chimeVolume: 0.7,
	chimeSingleFilePath: '/chime/chime1.mp3',
	chimeDoubleFilePath: '/chime/chime2.mp3',
	pagingIntervalSeconds: 8,
	emptySeatColor: '#F3E8DC',
	primaryTextColor: '#3a1e86',
	secondaryTextColor: '#f1e8f2',
	memberSeatWorkNameWidthPercent: 60,
	memberBigIconSize: 57.598,
	memberSmallIconSize: 38.391,
	menuIconSize: 45,
}

export const debug = false
