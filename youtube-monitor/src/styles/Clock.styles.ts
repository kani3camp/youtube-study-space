import { css } from '@emotion/react'

export const clockStyle = css`
    height: 160px;
    width: 400px;
    background-color: rgba(255, 241, 221, 1);
    backdrop-filter: blur(3px);
    position: absolute;
    top: 0;
    right: 0;
    color: #2d2b28;
`

export const dateStringStyle = css`
    font-size: 1.2rem;
    text-align: center;
`

export const timeStringStyle = css`
    font-size: 2rem;
    text-align: center;
    font-weight: bold;
`
