import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const roomLayout = css`
    position: relative;
    top: 0;
    left: 0;
    width: 100%;
    height: calc(1080px - 80px);
    box-sizing: border-box;
    margin: auto;
    /* border: solid 6px #303030; */
    background-size: contain;
`

export const seat = css`
    position: absolute;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    transform-origin: top left;
    font-family: ${Constants.seatFontFamily};
`

export const partition = css`
    position: absolute;
    background-color: #2d2b41;
`

export const seatId = css`
    margin: 0;
    position: relative;
    top: 0;
    left: 0;
    color: #414141;
`
export const seatIdContainerMember = css`
    position: absolute;
    top: 0;
    left: 0;
    width: 20%;
    height: 30%;
    background-color: yellow;
    border-right: solid black 0.06rem;
    border-bottom: solid black 0.06rem;
    border-bottom-right-radius: 40%;
`
export const seatIdMember = css`
    position: absolute;
    margin: 0;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: #414141;
`

export const emptySeatNum = css`
    margin: 0;
    color: #414141;
`
export const usedSeatNum = css``

export const workName = css`
    margin: 0;
    color: #24317e;
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    font-weight: bolder;
`
export const workNameMember = css`
    margin: 0;
    color: #24317e;
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    font-weight: bolder;
    position: absolute;
    width: 75%;
    right: 1%;
    top: 20%;
    text-align: center;
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
    width: 75%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    color: #202020;
    right: 1%;
    bottom: 20%;
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

export const starsBadge = css`
    color: black;
    position: absolute;
    top: 0;
    right: 0;
    overflow-wrap: anywhere;
`

export const profileImageMember = css`
    margin: 0;
    position: absolute;
    bottom: 20%;
    left: 5%;
    /* transform: translate(0, 50%); */
    border-radius: 50%;
`

export const timeElapsed = css`
    color: darkorange;
    position: absolute;
    bottom: 1%;
    left: 2%;
`

export const timeRemaining = css`
    color: green;
    position: absolute;
    bottom: 1%;
    right: 2%;
`
