import Image from 'next/image'
import type { FC } from 'react'
import type { RoomSceneConfig } from '../types/room-scene'

export type RoomVisualProps = {
	floorImage: string
	width: number
	height: number
	scene?: RoomSceneConfig
}

/**
 * Compatibility boundary for room visuals.
 *
 * Static room rendering remains authoritative in this PR. The optional scene
 * prop is accepted so later ambient/living renderers can be introduced behind
 * this component without changing seat/layout ownership.
 */
const RoomVisual: FC<RoomVisualProps> = ({
	floorImage,
	width,
	height,
	scene,
}) => {
	if (!floorImage) {
		return null
	}

	const ambientStyle =
		scene?.mode === 'ambient'
			? {
					filter: 'var(--scene-ambient-filter, none)',
					transition: 'filter 1s linear',
				}
			: undefined

	return (
		<Image
			alt="room image"
			src={floorImage}
			width={width}
			height={height}
			priority={true}
			style={ambientStyle}
		/>
	)
}

export default RoomVisual
