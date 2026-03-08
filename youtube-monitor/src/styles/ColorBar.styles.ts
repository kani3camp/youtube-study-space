import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import {
	Constants,
	sidebarBgmHeight,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

/** 目盛り（メモリ）の高さ。バー上端で止まるよう labelBarGapPx と調整 */
const scaleTickHeightPx = 7
/** ラベルとバーの間の隙間（メモリがバーに食い込まないようにする） */
const labelBarGapPx = 6

export const shape = css`
    height: ${Constants.colorBarHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    bottom: ${sidebarBgmHeight}px;
    right: 0;
`

export const colorBar = css`
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    border-radius: 0.6rem;
    padding: 0.25rem 0.7rem;
    box-sizing: border-box;
`

export const title = css`
    text-align: center;
    color: ${Constants.primaryTextColor};
    font-size: 0.58rem;
    font-weight: 500;
    margin: 0 0 0.12rem 0;
    font-family: ${fontFamily};
`

export const scaleWrapper = css`
    position: relative;
    width: calc(100% - 1.2rem);
    padding-top: 0.8rem;
    margin: 0 0.6rem;
    box-sizing: border-box;
`

export const labels = css`
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 0.75rem;
`

export const label = css`
    position: absolute;
    bottom: 1px;
    font-size: 0.4rem;
    font-weight: 500;
    color: #4b5563;
    font-family: ${fontFamily};
    transform: translateX(-50%);

    /* メモリはバーとの隙間内に収め、バー内には食い込まない（下端がバー上端で止まる） */
    &::after {
        content: '';
        position: absolute;
        bottom: -${scaleTickHeightPx}px;
        left: 50%;
        transform: translateX(-50%);
        width: 2.5px;
        height: ${scaleTickHeightPx}px;
        background-color: #6b7280;
        border-radius: 1px;
    }
`

/** left を 0〜100 の百分率で指定（例: 0, 20, 40, 60, 80, 93.33） */
export const labelPosition = (leftPercent: number) => css`
    left: ${leftPercent}%;
`

export const colorBarStrip = css`
    display: flex;
    width: 100%;
    height: 12px;
    margin-top: ${labelBarGapPx}px;
    border-radius: 5px;
    overflow: hidden;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.1);
`

export const colorBox = css`
    flex: 1;
    height: 100%;
`
