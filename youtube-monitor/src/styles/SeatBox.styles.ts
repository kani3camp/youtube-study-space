import { css } from '@emotion/react'
import { Constants } from '../lib/constants'
import { fontFamily } from '../lib/common'

export const emptySeatNum = css`
    margin: 0;
    color: #414141;
`
export const usedSeatNum = css``

export const seat = css`
    position: absolute;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    transform-origin: top left;
    font-family: ${fontFamily};
`

export const seatId = css`
    margin: 0;
    position: relative;
    color: #414141;
    font-weight: bold;
`
export const seatIdMember = css`
    position: absolute;
    margin: 0;
    top: 0;
    color: #414141;
    font-weight: bold;
`

export const workName = css`
    margin: 0;
    color: #24317e;
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    font-weight: bolder;
`
export const workNameMemberBalloon = css`
    position: absolute;
    bottom: 38%;
    left: 23%;
    text-align: center;
    min-width: 30%;
    max-width: ${Constants.memberSeatWorkNameWidthPercent}%;
    max-height: 1.65rem;
    margin-left: 0.5rem;
    padding: 0.08rem 0.2rem;
    border-radius: 0.3rem;
    background-color: #ffffffa0;
    color: #24317e;
    font-weight: bolder;

    &::before {
        position: absolute;
        bottom: 25%;
        left: -0.5rem;
        width: 0.5rem;
        height: 0.4rem;
        clip-path: polygon(0 50%, 100% 0, 100% 100%);
        content: '';
        background-color: inherit;
    }
`

export const workNameMemberText = css`
    max-height: inherit;
    text-overflow: ellipsis;
    overflow: hidden;
    word-wrap: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
`

export const userDisplayName = css`
    margin: 0;
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    color: #202020;
`
export const userDisplayNameMember = css`
    position: absolute;
    margin: 0;
    width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    color: #202020;
    right: 0;
    bottom: 14%;
    text-align: center;
`

export const breakBadge = css`
    color: black;
    background-color: #5af87fe7;
    z-index: ${Constants.breakBadgeZIndex};
    position: absolute;
    font-weight: bold;
    border: solid;
`

export const menuItem = css`
    position: absolute;
    top: 0.1rem;
    left: 0.2rem;
`

export const starsBadge = css`
    color: black;
    position: absolute;
    top: 0;
    right: 0;
    overflow-wrap: anywhere;
    padding-right: 0.2rem;
`

export const profileImageMemberWithWorkName = css`
    margin: 0;
    position: absolute;
    top: 35%;
    left: 12%;
    width: 1.2rem;
    height: 1.2rem;
    transform: translate(-50%, 0);
    border-radius: 50%;
`
export const profileImageMemberNoWorkName = css`
    margin: 0;
    position: absolute;
    top: 24%;
    left: 50%;
    width: 1.8rem;
    height: 1.8rem;
    transform: translate(-50%, 0);
    border-radius: 50%;
`

export const timeElapsed = css`
    color: #000;
    position: absolute;
    bottom: 1%;
    left: 2%;
`

export const timeRemaining = css`
    color: black;
    position: absolute;
    bottom: 1%;
    right: 2%;
`
