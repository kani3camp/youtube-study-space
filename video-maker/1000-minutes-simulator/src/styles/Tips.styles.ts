import { css } from '@emotion/react'
import {
	InnerMargin,
	OuterMargin,
	TipsTextFontFamily,
	basicCell,
	basicInnerCell,
	iconBase,
} from './common.styles'

const TipsTextFontSize = 35
const TipsPrefixWidth = 180

export const tips = css`
    ${basicCell};
    grid-column-start: 1;
    grid-column-end: 4;
    grid-row-start: 2;
    grid-row-end: 3;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${2 * InnerMargin}px);
    width: calc(100% - ${InnerMargin + OuterMargin}px);
    margin: ${InnerMargin}px ${InnerMargin}px ${InnerMargin}px ${OuterMargin}px;
    span {
        margin: 0.2rem 0.3rem;
    }
`

export const baseContainer = css`
    margin: 0.3rem;
    display: flex;
    align-items: center;
`

export const tipsTextContainer = css`
    ${baseContainer};
    height: ${3 * TipsTextFontSize}px;
`

export const tipsTextPrefix = css`
    width: 180px;
`

export const tipsText = css`
    flex: 1;
    font-size: ${TipsTextFontSize}px;
    font-family: ${TipsTextFontFamily};
    color: yellow;
`

export const tipsPosterContainer = css`
    ${baseContainer};
`
export const tipsPosterPrefix = css`
    width: ${TipsPrefixWidth}px;
`
export const tipsPoster = css`
    flex: 1;
`

export const tipsNoteContainer = css`
    ${baseContainer};
`
export const tipsNotePrefix = css`
    width: ${TipsPrefixWidth}px;
`
export const tipsNote = css`
    flex: 1;
    font-size: 0.8rem;
`

export const icon = css`
    ${iconBase};
    color: #f9f954;
`
