import { css } from '@emotion/react'


export const standingRoom = css`
  height: 350px;
  width: 400px;
  padding: 0.4rem;
  box-sizing: border-box;
  background-color: rgba(239, 255, 248, 0.829);
  position: absolute;
  top: 200px;
  right: 0;
  font-size: 1rem;
  text-align: center;
  color: #383838;
`

export const seatId = css`
  margin: 0 auto;
  font-size: 0.7em;
  color: #414141;
`

export const description = css`
  margin: 0.2rem;
`

export const seat = css`
  border: solid #3a3a3a 0.09rem;
  width: 7.1rem;
  height: 3.5rem;
  margin: 0.2rem auto;
  margin-bottom: 1rem;
  text-overflow: ellipsis;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  border-radius: 0.15rem;
`

export const workName = css`
  margin: 0;
  font-size: 0.8em;
  color: #24317e;
  max-width: 100%;
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
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

export const commandString = css`
  font-weight: bold;
  display: inline-block;
  background-color: #e7e7e7;
  border: solid #6464645d 0.05rem;
  border-radius: 0.2rem;
  padding: 0 0.4rem;
  margin: 0.1rem;
`