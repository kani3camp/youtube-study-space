import { useRouter } from 'next/router'
import { useEffect, useRef, useState } from 'react'
import { DEBUG } from '../lib/constants'
import {
	resolveThemePresentation,
	shouldUseContrastBridge,
	type ThemePresentation,
} from '../lib/time-theme'

const THEME_UPDATE_INTERVAL_MS = 30_000
export const CONTRAST_BRIDGE_DURATION_MS = 15_000
const HYDRATION_SAFE_PRESENTATION: ThemePresentation = {
	timeTheme: 'day',
	textTone: 'dark',
}

export type ActiveThemePresentation = ThemePresentation & {
	contrastBridge: boolean
}

export function useTimeTheme(): ActiveThemePresentation {
	const router = useRouter()
	const queryTheme = router.query.timeTheme
	const queryTextTone = router.query.textTone
	const [presentation, setPresentation] = useState<ThemePresentation>(
		HYDRATION_SAFE_PRESENTATION,
	)
	const [contrastBridge, setContrastBridge] = useState(false)
	const presentationRef = useRef(presentation)

	useEffect(() => {
		if (!router.isReady) {
			return
		}

		let toneSwitchTimeoutId: number | undefined
		const commitPresentation = (next: ThemePresentation) => {
			presentationRef.current = next
			setPresentation(next)
		}

		const updateTheme = () => {
			const target = resolveThemePresentation(
				new Date(),
				queryTheme,
				queryTextTone,
				DEBUG,
			)
			const current = presentationRef.current
			if (
				current.timeTheme === target.timeTheme &&
				current.textTone === target.textTone
			) {
				setContrastBridge(false)
				return
			}

			if (toneSwitchTimeoutId !== undefined) {
				window.clearTimeout(toneSwitchTimeoutId)
				toneSwitchTimeoutId = undefined
			}

			if (!shouldUseContrastBridge(current, target)) {
				setContrastBridge(false)
				commitPresentation(target)
				return
			}

			setContrastBridge(true)
			commitPresentation({
				timeTheme: target.timeTheme,
				textTone: current.textTone,
			})
			toneSwitchTimeoutId = window.setTimeout(() => {
				const latest = presentationRef.current
				if (latest.timeTheme !== target.timeTheme) {
					return
				}
				commitPresentation({ ...latest, textTone: target.textTone })
				setContrastBridge(false)
				toneSwitchTimeoutId = undefined
			}, CONTRAST_BRIDGE_DURATION_MS)
		}

		updateTheme()
		const intervalId = window.setInterval(updateTheme, THEME_UPDATE_INTERVAL_MS)
		return () => {
			window.clearInterval(intervalId)
			if (toneSwitchTimeoutId !== undefined) {
				window.clearTimeout(toneSwitchTimeoutId)
			}
		}
	}, [queryTextTone, queryTheme, router.isReady])

	return { ...presentation, contrastBridge }
}
