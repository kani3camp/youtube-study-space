import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

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
