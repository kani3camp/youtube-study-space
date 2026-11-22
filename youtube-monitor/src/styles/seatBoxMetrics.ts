/**
 * SeatBox の幅計算（SeatBox.tsx）と CSS（SeatBox.styles.ts）で同期が必要な数値定数。
 * CSS 側でもテンプレートリテラルで参照しているため、ここを変更すると両方に反映される。
 */

/** .seat の border（片側、px） */
export const seatBorderWidthPx = 1 as const

/** seatBody の左右 padding（em） */
export const seatBodyHorizontalPaddingEm = 0.5 as const
