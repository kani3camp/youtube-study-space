import { css } from '@emotion/react'

export const message = css`
    height: 80px;
    width: calc(1920px - 400px);
    position: absolute;
    bottom: 0;
    left: 0;
    text-align: center;
    font-size: 1.6rem;
    background-color: #ffffff60;
    display: flex;
    flex-direction: row;
`

export const pageInfo = css`
    width: 30%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
`

export const pageIndex = css`
    display: inline-block;
`

export const memberOnly = css`
    width: 2.5rem;
    margin-left: 1rem;
    padding: 0.1rem;
    display: inline-block;
    font-size: 0.6rem;
    color: white;
    background-color: #2ba640;
    border-radius: 0.3rem;
`

export const numStudyingPeople = css`
    width: 70%;
    height: 100%;
    display: inline-block;
    background-color: rgba(255, 241, 221, 0.9);
    border-radius: 1rem 0 0 0;
`
