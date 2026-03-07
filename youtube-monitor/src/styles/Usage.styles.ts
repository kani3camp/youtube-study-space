import { css } from '@emotion/react'
import { sourceCodeProFontFamily } from '../lib/common'
import {
	Constants,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

export const shape = css`
    height: ${Constants.usageHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: ${Constants.clockHeight}px;
    right: 0;
`

export const usage = css`
    font-size: 1rem;
    text-align: center;
    color: ${Constants.primaryTextColor};
    box-sizing: border-box;
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    padding: 0.4rem;
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 0.35rem;
`

export const description = css`
    margin: 0.2rem;
    font-weight: bold;
`

export const command = css`
    margin: 0.0rem;
`

export const commandCode = css`
    font-family: ${sourceCodeProFontFamily};
    font-weight: 700;
    font-size: 0.88rem;
    letter-spacing: 0.01em;
    font-variant-ligatures: none;
    display: inline-block;
    background-color: rgba(0, 0, 0, 0.08);
    border-radius: 0.22rem;
    padding: 0.12rem 0.5rem;
    margin: 0.0rem 0.18rem;
    line-height: 1.35;
`

export const commandDesc = css`
    font-size: 0.9rem;
`
