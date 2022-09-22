import { useEffect, useRef } from 'react'
import { RoomsStateResponse } from '../types/api'

export const useInterval = (
    callback: () => void,
    intervalMilliSec: number
): void => {
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

export const validateRoomsStateResponse = (
    resp: RoomsStateResponse
): boolean => {
    if (resp) {
        // pass
    } else {
        return false
    }
    if (resp.max_seats && resp.max_seats > 0) {
        // pass
    } else {
        return false
    }
    if (resp.result && resp.result === 'ok') {
        // pass
    } else {
        return false
    }
    if (
        resp.min_vacancy_rate &&
        resp.min_vacancy_rate > 0 &&
        resp.min_vacancy_rate < 1
    ) {
        // pass
    } else {
        return false
    }
    if (resp.seats && resp.seats.length >= 0) {
        // pass
    } else {
        return false
    }

    return true
}
