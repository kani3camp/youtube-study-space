import { useRouter } from 'next/router'
import { type RefObject, useEffect } from 'react'
import { DEBUG } from '../lib/constants'
import {
	buildAmbientSceneFilter,
	createSceneClockAnchor,
	getSceneClockSecondsAt,
	getSceneStateAtClockSeconds,
} from '../lib/scene-clock'

const SCENE_CLOCK_UPDATE_INTERVAL_MS = 1000

const SCENE_VARIABLES = [
	'--scene-ambient-filter',
	'--scene-brightness',
	'--scene-saturation',
	'--scene-warmth',
	'--scene-sunset-amount',
	'--scene-night-amount',
	'--scene-lamp-intensity',
	'--scene-sky-brightness',
] as const

export function useSceneClockCssVariables(
	targetRef: RefObject<HTMLElement | null>,
): void {
	const router = useRouter()
	const sceneTimeQuery = router.query.sceneTime
	const sceneSpeedQuery = router.query.sceneSpeed

	useEffect(() => {
		if (!router.isReady || targetRef.current === null) {
			return
		}

		const target = targetRef.current
		const anchor = createSceneClockAnchor(
			new Date(),
			sceneTimeQuery,
			sceneSpeedQuery,
			DEBUG,
		)

		const update = () => {
			const clockSeconds = getSceneClockSecondsAt(anchor, Date.now())
			const state = getSceneStateAtClockSeconds(clockSeconds)
			target.style.setProperty(
				'--scene-ambient-filter',
				buildAmbientSceneFilter(state),
			)
			target.style.setProperty('--scene-brightness', String(state.brightness))
			target.style.setProperty('--scene-saturation', String(state.saturation))
			target.style.setProperty('--scene-warmth', String(state.warmth))
			target.style.setProperty('--scene-sunset-amount', String(state.sunsetAmount))
			target.style.setProperty('--scene-night-amount', String(state.nightAmount))
			target.style.setProperty('--scene-lamp-intensity', String(state.lampIntensity))
			target.style.setProperty('--scene-sky-brightness', String(state.skyBrightness))
		}

		update()
		const intervalId = window.setInterval(update, SCENE_CLOCK_UPDATE_INTERVAL_MS)
		return () => {
			window.clearInterval(intervalId)
			for (const variable of SCENE_VARIABLES) {
				target.style.removeProperty(variable)
			}
		}
	}, [router.isReady, sceneSpeedQuery, sceneTimeQuery, targetRef])
}
