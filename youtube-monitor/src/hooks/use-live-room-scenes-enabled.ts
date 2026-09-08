import { useRouter } from 'next/router'
import { DEBUG } from '../lib/constants'
import {
	LIVE_ROOM_SCENES_ENABLED_BY_DEFAULT,
	resolveLiveRoomScenesEnabled,
} from '../lib/live-room-scenes'

export function useLiveRoomScenesEnabled(): boolean {
	const router = useRouter()
	return resolveLiveRoomScenesEnabled({
		defaultEnabled: LIVE_ROOM_SCENES_ENABLED_BY_DEFAULT,
		queryValue: router.query.liveScenes,
		debugEnabled: DEBUG,
		routerReady: router.isReady,
	})
}
