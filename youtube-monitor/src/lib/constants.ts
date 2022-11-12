if (process.env.NEXT_PUBLIC_DEBUG !== 'true' && process.env.NEXT_PUBLIC_DEBUG !== 'false') {
    throw Error(`invalid NEXT_PUBLIC_DEBUG: ${process.env.NEXT_PUBLIC_DEBUG?.toString()}`)
}
export const DEBUG = process.env.NEXT_PUBLIC_DEBUG === 'true'

export const Constants = {
    fontFamily: "'Zen Maru Gothic', sans-serif",
    breakBadgeZIndex: 10,
    seatFontFamily: "'M PLUS Rounded 1c', sans-serif",
    bgmVolume: DEBUG ? 0.03 : 0.3,
    chimeVolume: 0.7,
    chime1FilePath: '/chime/chime1.mp3',
    chime2FilePath: '/chime/chime2.mp3',
    pagingIntervalSeconds: 8,
}
