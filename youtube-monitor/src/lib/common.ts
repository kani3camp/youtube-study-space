import { useEffect, useRef } from 'react'

export const useInterval = (callback: () => void, intervalMilliSec: number) => {
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
    })
}
