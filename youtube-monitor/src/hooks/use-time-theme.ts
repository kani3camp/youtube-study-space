import { useRouter } from 'next/router'
import { useEffect, useState } from 'react'
import { DEBUG } from '../lib/constants'
import { resolveTimeTheme, type TimeTheme } from '../lib/time-theme'

const THEME_UPDATE_INTERVAL_MS = 30_000
const HYDRATION_SAFE_THEME: TimeTheme = 'day'

export function useTimeTheme(): TimeTheme {
	const router = useRouter()
	const queryTheme = router.query.timeTheme
	const [theme, setTheme] = useState<TimeTheme>(HYDRATION_SAFE_THEME)

	useEffect(() => {
		if (!router.isReady) {
			return
		}

		const updateTheme = () => {
			const nextTheme = resolveTimeTheme(new Date(), queryTheme, DEBUG)
			setTheme((currentTheme) =>
				currentTheme === nextTheme ? currentTheme : nextTheme,
			)
		}

		updateTheme()
		const intervalId = window.setInterval(updateTheme, THEME_UPDATE_INTERVAL_MS)
		return () => {
			window.clearInterval(intervalId)
		}
	}, [queryTheme, router.isReady])

	return theme
}
