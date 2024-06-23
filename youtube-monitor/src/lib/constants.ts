if (process.env.NEXT_PUBLIC_DEBUG !== 'true' && process.env.NEXT_PUBLIC_DEBUG !== 'false') {
    throw Error(`invalid NEXT_PUBLIC_DEBUG: ${process.env.NEXT_PUBLIC_DEBUG?.toString()}`)
}
if (
    process.env.NEXT_PUBLIC_CHANNEL_GL !== 'true' &&
    process.env.NEXT_PUBLIC_CHANNEL_GL !== 'false'
) {
    throw Error(`invalid NEXT_PUBLIC_CHANNEL_GL: ${process.env.NEXT_PUBLIC_CHANNEL_GL?.toString()}`)
}
export const DEBUG = process.env.NEXT_PUBLIC_DEBUG === 'true'
export const CHANNEL_GL = process.env.NEXT_PUBLIC_CHANNEL_GL === 'true'

export const Constants = {
    screenWidth: 1920,
    screenHeight: 1080,
    sideBarWidth: 400,
    messageBarHeight: 80,
    clockHeight: 160,
    usageHeight: 390,
    timerHeight: 330,
    fontFamily: "'M PLUS Rounded 1c', sans-serif",
    breakBadgeZIndex: 10,
    seatFontFamily: "'M PLUS Rounded 1c', sans-serif",
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
}

export const debug = false
