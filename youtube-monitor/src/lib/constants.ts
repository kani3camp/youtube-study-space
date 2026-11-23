export const roomConfigNames = ['PROD', 'DEV'] as const
export type RoomConfigName = (typeof roomConfigNames)[number]

export const parseRoomConfigName = (
	value: string | undefined,
): RoomConfigName => {
	if (value === 'PROD' || value === 'DEV') {
		return value
	}
	throw Error(`invalid NEXT_PUBLIC_ROOM_CONFIG: ${value?.toString()}`)
}

export const parseDebugFlag = (value: string | undefined): boolean => {
	if (value === 'true') {
		return true
	}
	if (value === 'false') {
		return false
	}
	throw Error(`invalid NEXT_PUBLIC_DEBUG: ${value?.toString()}`)
}

// NEXT_PUBLIC_* はビルド時にリテラルアクセスが置換されるため、直接参照する。
// 読み込み時に評価せず、利用時に検証する。
export const isDebugEnabled = (): boolean =>
	parseDebugFlag(process.env.NEXT_PUBLIC_DEBUG)

export const getRoomConfigName = (): RoomConfigName =>
	parseRoomConfigName(process.env.NEXT_PUBLIC_ROOM_CONFIG)

export const getBgmVolume = (): number => (isDebugEnabled() ? 0.1 : 0.3)

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
