import { css } from '@emotion/react'

export const FontFamily = "'M PLUS Rounded 1c', sans-serif"

export const ScreenHeight = 1080
export const ScreenWidth = 1920

export const GaugeHeight = ScreenHeight
export const GaugeWidth = 500

export const TimerHeight = 400
export const TimerWidth = (ScreenWidth - GaugeWidth) / 3

export const BGMPlayerHeight = 250
export const BGMPlayerWidth = ScreenWidth - GaugeWidth

export const TipsHeight = ScreenHeight - TimerHeight - BGMPlayerHeight
export const TipsWidth = ScreenWidth - GaugeWidth
export const TipsTop = TimerHeight

export const ElapsedHeight = TimerHeight
export const ElapsedWidth = TimerWidth
export const ElapsedLeft = TimerWidth

export const CurrentColorHeight = TimerHeight
export const CurrentColorWidth =
    ScreenWidth - TimerWidth - ElapsedWidth - GaugeWidth
export const CurrentColorLeft = TimerWidth + ElapsedWidth
export const ColorBoxHeight = 170
export const ColorBoxWidth = 170

export const basicCell = css`
    background-color: gray;
`

export const basicInnerCell = css`
    height: 100%;
    width: 100%;
    padding: 0.5rem;
    margin: 0.5rem;
    border-radius: 0.5rem;
    background-color: #111111;
`

export const heading = css`
    font-weight: bold;
    font-size: 1.3rem;
`
