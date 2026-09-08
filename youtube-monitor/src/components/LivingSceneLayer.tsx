import type { FC } from 'react'
import { useEffect, useRef } from 'react'
import { livingSceneRuntime } from '../lib/living-scene-runtime'
import * as styles from '../styles/LivingSceneLayer.styles'

type LivingSceneLayerProps = {
	active: boolean
	width: number
	height: number
}

const LivingSceneLayer: FC<LivingSceneLayerProps> = ({
	active,
	width,
	height,
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

		void livingSceneRuntime.activate({ host, width, height })
		return () => {
			livingSceneRuntime.deactivate(host)
		}
	}, [active, height, width])

	return <div ref={hostRef} css={styles.host} aria-hidden="true" />
}

export default LivingSceneLayer
