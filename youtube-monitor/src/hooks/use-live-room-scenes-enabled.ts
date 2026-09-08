import { useRouter } from 'next/router'
import { useEffect, useState } from 'react'
import { DEBUG } from '../lib/constants'
import {
	LIVE_ROOM_SCENES_ENABLED_BY_DEFAULT,
	resolveLiveRoomScenesEnabled,
} from '../lib/live-room-scenes'

export function useLiveRoomScenesEnabled(): boolean {
	const router = useRouter()
	const liveScenesQuery = router.query.liveScenes
	const [enabled, setEnabled] = useState(false)

	useEffect(() => {
		setEnabled(
			resolveLiveRoomScenesEnabled({
				defaultEnabled: LIVE_ROOM_SCENES_ENABLED_BY_DEFAULT,
				queryValue: liveScenesQuery,
				debugEnabled: DEBUG,
				routerReady: router.isReady,
			}),
		)
	}, [liveScenesQuery, router.isReady])

	return enabled
}
