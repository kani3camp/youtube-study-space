import type { FC } from 'react'
import { useEffect, useRef } from 'react'
import { livingSceneRuntime } from '../lib/living-scene-runtime'
import * as styles from '../styles/LivingSceneLayer.styles'
import type { LivingSceneProfile } from '../types/room-scene'

type LivingSceneLayerProps = {
	active: boolean
	width: number
	height: number
	profile: LivingSceneProfile
}

const LivingSceneLayer: FC<LivingSceneLayerProps> = ({
	active,
	width,
	height,
	profile,
}) => {
	const hostRef = useRef<HTMLDivElement>(null)

	useEffect(() => {
		const host = hostRef.current
		if (host === null) {
			return
		}

		if (!active) {
			livingSceneRuntime.deactivate(host)
			return
		}

		void livingSceneRuntime.activate({ host, width, height, profile })
		return () => {
			livingSceneRuntime.deactivate(host)
		}
	}, [active, height, profile, width])

	return <div ref={hostRef} css={styles.host} aria-hidden="true" />
}

export default LivingSceneLayer
