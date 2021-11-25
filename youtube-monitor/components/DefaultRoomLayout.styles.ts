import { css } from '@emotion/react'


export const roomLayout = css`
  position: relative;
  top: 0;
  left: 0;
  width: 100%;
  height: calc(1080px - 70px);
  padding: 0.3rem;
  box-sizing: border-box;
  margin: auto;
  border: solid 6px #303030;
  display: flex;
  flex-wrap: wrap;
  overflow: hidden;
`

export const seat = css`
  width: 7.3rem;
  height: 3.5rem;
  margin: 0.2rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  border-radius: 0.15rem;
  // border: solid black 0.03rem;
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
  font-size: 0.7em;
  font-weight: bold;
  color: #414141;
`

export const usedSeatNum = css`
  font-size: 0.7em;
`

export const workName = css`
  margin: 0;
  font-size: 0.9em;
  color: #24317e;
  max-width: 100%;
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  font-weight: bolder;
`

export const userDisplayName = css`
  margin: 0;
  font-size: 1em;
  max-width: 100%;
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  color: #202020;
`
