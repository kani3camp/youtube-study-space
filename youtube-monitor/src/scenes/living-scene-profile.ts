import type { Container, Graphics, Ticker } from 'pixi.js'
import type { LivingSceneProfile } from '../types/room-scene'
import { createLumeRainyScene } from './lume-rainy-poc'

export type LivingSceneFactoryContext = {
	root: Container
	ticker: Ticker
	makeContainer: () => Container
	makeGraphics: () => Graphics
	getHost: () => HTMLElement | null
}

export type LivingSceneInstance = {
	resize: (width: number, height: number) => void
	destroy: () => void
}

export function createLivingSceneForProfile(
	profile: LivingSceneProfile,
	context: LivingSceneFactoryContext,
): LivingSceneInstance | undefined {
	switch (profile) {
		case 'lume-rainy-poc':
			return createLumeRainyScene(context)
		default:
			return undefined
	}
}
