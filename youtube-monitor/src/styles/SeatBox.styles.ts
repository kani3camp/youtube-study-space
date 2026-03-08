import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import { Constants } from '../lib/constants'

export const seat = css`
    position: absolute;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    transform-origin: top left;
    font-family: ${fontFamily};
    overflow: hidden;
    box-sizing: border-box;
`

export const topBar = css`
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    z-index: 1;
`

export const seatNumber = css`
    position: absolute;
    color: #999;
    font-weight: 600;
    z-index: 2;
`

export const workName = css`
    color: #24317e;
    font-weight: bold;
    text-align: center;
    max-width: 90%;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    word-break: break-word;
    line-height: 1.25;
`

export const userName = css`
    color: #555;
    text-align: center;
    max-width: 90%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
`

export const userNameRow = css`
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    max-width: 90%;
    overflow: hidden;
`

export const emptyNumber = css`
    color: #999;
    font-weight: bold;
`

export const emptyLabel = css`
    color: #bbb;
    font-weight: 500;
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
    z-index: 3;
`

export const starsBadge = css`
    color: black;
    position: absolute;
    top: 0;
    right: 0;
    overflow-wrap: anywhere;
    padding-right: 0.2rem;
    z-index: 2;
`

export const profileImageSmall = css`
    border-radius: 50%;
    flex-shrink: 0;
`

export const profileImageLarge = css`
    border-radius: 50%;
`

export const timeElapsed = css`
    color: #999;
    position: absolute;
    bottom: 3%;
    left: 5%;
`

export const timeRemaining = css`
    color: #555;
    font-weight: 600;
    position: absolute;
    bottom: 3%;
    right: 5%;
`
