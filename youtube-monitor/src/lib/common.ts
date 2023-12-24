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
