import { css, keyframes } from '@emotion/react'
import { Constants } from '../lib/constants'

const centerLoadingSpin = keyframes`
    to {
        transform: rotate(360deg);
    }
`

export const CenterLoading = css`
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    width: 100%;
    display: flex;
    justify-content: center;
    align-items: center;
    flex-direction: column;
    color: ${Constants.primaryTextColor};
    font-size: 1.5rem;
`

export const spinner = css`
    width: 72px;
    height: 72px;
    margin-bottom: 16px;
    border: 6px solid rgba(255, 255, 255, 0.2);
    border-top-color: ${Constants.primaryTextColor};
    border-radius: 9999px;
    animation: ${centerLoadingSpin} 0.9s linear infinite;
`
