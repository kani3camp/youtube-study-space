import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import { Constants } from '../lib/constants'
import {
	seatDisplayNameFontWeight,
	seatIdFontWeight,
	seatWorkNameFontWeight,
} from './seatBoxFontWeights'

export const seat = css`
    position: absolute;
    display: flex;
    flex-direction: column;
    transform-origin: top left;
    box-sizing: border-box;
    overflow: hidden;
    border: 1px solid rgba(84, 75, 62, 0.12);
    box-shadow: 0 0.2rem 0.55rem rgba(70, 58, 43, 0.08);
    font-family: ${fontFamily};
`

export const accentBar = css`
    position: absolute;
    top: 0;
    left: 0;
    display: flex;
    align-items: flex-start;
    justify-content: flex-end;
    width: 100%;
    box-sizing: border-box;
    pointer-events: none;
`

export const seatBody = css`
    position: relative;
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    padding: 0.35em 0.5em 0.34em;
`

export const headerRow = css`
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    height: 0.7rem;
`

export const headerLeft = css`
    display: flex;
    align-items: center;
    align-self: stretch;
    min-width: 0;
    gap: 0.2rem;
`

export const generalSeatId = css`
    margin: 0;
    color: #5a5146;
    line-height: 1;
    font-size: 0.6em;
    font-weight: ${seatIdFontWeight};
    letter-spacing: 0.02em;
`

export const memberSeatId = css`
    margin: 0;
    color: #5a5146;
    line-height: 1;
    font-size: 0.68em;
    font-weight: ${seatIdFontWeight};
    letter-spacing: 0.02em;
`

export const menuItem = css`
    flex-shrink: 0;
    opacity: 0.72;
`

export const breakBadge = css`
    color: #2f3d19;
    background-color: rgba(199, 230, 146, 0.95);
    z-index: ${Constants.breakBadgeZIndex};
    font-weight: ${seatDisplayNameFontWeight};
    line-height: 1;
    border: 1px solid rgba(67, 90, 29, 0.22);
`

export const starsBadge = css`
    color: rgba(77, 63, 19, 0.96);
    position: absolute;
    top: 0.2rem;
    right: 0.2rem;
    font-weight: ${seatWorkNameFontWeight};
    line-height: 1;
    white-space: nowrap;
    transform: translateY(1px);
`

export const memberContent = css`
    position: relative;
    display: flex;
    gap: 0.2rem;
    flex: 1;
    flex-direction: column;
    min-height: 0;
`

export const memberMain = css`
    display: flex;
    flex: 1;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    gap: 0.14em;
    min-height: 0;
    text-align: center;
    margin-top: auto;
`

export const emptyContent = css`
    display: flex;
    flex: 1;
    justify-content: center;
    align-items: center;
    min-height: 0;
    text-align: center;
`

export const emptySeatCommand = css`
    margin: 0;
    color: #4a4338;
    line-height: 1;
    font-weight: ${seatWorkNameFontWeight};
    letter-spacing: 0.02em;
`

export const memberWorkNameFrame = css`
    align-self: center;
    min-width: 50%;
    padding: 0.1em 0.22em;
    box-sizing: border-box;
    border-radius: 0.3rem;
    background-color: rgba(255, 255, 255, 0.63);
    text-align: center;
    padding-top: 0.2em;

`

export const memberWorkName = css`
    color: #24317e;
    text-overflow: ellipsis;
    font-weight: ${seatWorkNameFontWeight};
    line-height: 1.18;
    overflow: hidden;
    word-wrap: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
`

export const generalContent = css`
    display: flex;
    flex: 1;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    min-height: 0;
    text-align: center;
`

export const generalWorkName = css`
    width: 100%;
    color: #24317e;
    font-weight: ${seatWorkNameFontWeight};
    line-height: 1.15;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
`

export const generalDisplayName = css`
    margin: 0;
    width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: rgb(75, 71, 66);
    font-weight: ${seatDisplayNameFontWeight};
    line-height: 1.15;
`

export const memberIdentityRow = css`
    display: flex;
    align-items: center;
    overflow: hidden;
    gap: 0.22em;
    max-width: 100%;
`

export const memberDisplayName = css`
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: rgb(75, 71, 66);
    font-weight: ${seatDisplayNameFontWeight};
    line-height: 1.15;
    text-align: center;
`

export const profileImage = css`
    border-radius: 999px;
    object-fit: cover;
    opacity: 0.92;
    flex-shrink: 0;
`

export const timeElapsed = css`
    color: rgba(46, 41, 36, 0.72);
    font-size: 0.42em;
    line-height: 1;
    white-space: nowrap;
    padding-left: 0;
    position: absolute;
    left: 0.2rem;
    bottom: 0.2rem;

`

export const timeRemaining = css`
    color: #2a241d;
    font-size: 0.42em;
    font-weight: ${seatWorkNameFontWeight};
    line-height: 1;
    white-space: nowrap;
    flex-shrink: 0;
    position: absolute;
    right: 0.2rem;
    bottom: 0.2rem;

`
