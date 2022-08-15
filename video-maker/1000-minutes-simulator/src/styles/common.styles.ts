import { css } from '@emotion/react'

export const OuterMargin = 20
export const InnerMargin = OuterMargin / 2

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
export const ColorBoxHeight = 150
export const ColorBoxWidth = 150

export const IconSize = 30

export const basicCell = css`
    background-color: #131313;
`

export const basicInnerCell = css`
    border-radius: 0.5rem;
    padding: 20px;
    background-color: #292a4b;
`

export const heading = css`
    color: #c9c9c9;
    font-weight: bold;
    font-size: ${IconSize}px;
    text-align: left;
    margin-bottom: 0.5rem;
`

export const iconBase = css`
    vertical-align: middle;
    margin-right: 0.5rem;
`
