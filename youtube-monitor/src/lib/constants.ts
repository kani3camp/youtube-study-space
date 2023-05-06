if (process.env.NEXT_PUBLIC_DEBUG !== 'true' && process.env.NEXT_PUBLIC_DEBUG !== 'false') {
    throw Error(`invalid NEXT_PUBLIC_DEBUG: ${process.env.NEXT_PUBLIC_DEBUG?.toString()}`)
}
export const DEBUG = process.env.NEXT_PUBLIC_DEBUG === 'true'

export const Constants = {
    screenWidth: 1920,
    screenHeight: 1080,
    sideBarWidth: 400,
    messageBarHeight: 80,
    fontFamily: "'Zen Maru Gothic', sans-serif",
    breakBadgeZIndex: 10,
    seatFontFamily: "'M PLUS Rounded 1c', sans-serif",
    bgmVolume: DEBUG ? 0.1 : 0.3,
    chimeVolume: 0.7,
    chimeSingleFilePath: '/chime/chime1.mp3',
    chimeDoubleFilePath: '/chime/chime2.mp3',
    pagingIntervalSeconds: 8,
    emptySeatColor: '#F3E8DC',
    memberSeatWorkNameWidthPercent: 58,
}
