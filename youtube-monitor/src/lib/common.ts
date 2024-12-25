import { useEffect, useRef } from 'react'
import { RoomLayout } from '../types/room-layout'

export const useInterval = (callback: () => void, intervalMilliSec: number): void => {
    const callbackRef = useRef<() => void>(callback)
    useEffect(() => {
        callbackRef.current = callback
    }, [callback])

    useEffect(() => {
        const tick = () => {
            callbackRef.current()
        }
        const id = setInterval(tick, intervalMilliSec)
        return () => {
            clearInterval(id)
        }
    }, [])
}

/**
 * Number of seats of the given layouts.
 * @param layouts
 * @returns
 */
export const numSeatsOfRoomLayouts = (layouts: RoomLayout[]) => {
    let count = 0
    for (const layout of layouts) {
        if (layout) {
            count += layout.seats.length
        }
    }
    return count
}

export const validateString = (value: string | undefined | null): boolean =>
    value !== undefined && value !== null && value !== ''

import { M_PLUS_Rounded_1c } from 'next/font/google'

const mPlusRounded1c = M_PLUS_Rounded_1c({
    subsets: ['latin'],
    weight: ['100', '300', '400', '500', '700', '800', '900'],
    display: 'swap',
})
const fontFamilyString = mPlusRounded1c.style.fontFamily
export const fontFamily = fontFamilyString.includes(' ')
    ? `'${fontFamilyString}'`
    : fontFamilyString
