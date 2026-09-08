import type { Graphics } from 'pixi.js'
import type {
	LivingSceneFactoryContext,
	LivingSceneInstance,
} from './living-scene-profile'

const DESIGN_WIDTH = 1520
const DESIGN_HEIGHT = 1000
const SCENE_STATE_REFRESH_MS = 1000

type WindowPane = {
	x: number
	y: number
	width: number
	height: number
}

export type LumeRainDropSpec = {
	x: number
	y: number
	minY: number
	maxY: number
	length: number
	speed: number
	alpha: number
	width: number
}

type AnimatedRainDrop = LumeRainDropSpec & {
	graphic: Graphics
}

const WINDOW_PANES: readonly WindowPane[] = [
	{ x: 122, y: 100, width: 95, height: 420 },
	{ x: 270, y: 75, width: 120, height: 400 },
	{ x: 450, y: 50, width: 100, height: 350 },
	{ x: 625, y: 30, width: 110, height: 320 },
	{ x: 850, y: 25, width: 120, height: 330 },
	{ x: 1015, y: 65, width: 85, height: 330 },
	{ x: 1225, y: 105, width: 85, height: 350 },
	{ x: 1350, y: 135, width: 48, height: 345 },
]

const LAMP_GLOWS = [
	{ x: 82, y: 260, radius: 80 },
	{ x: 235, y: 308, radius: 78 },
	{ x: 422, y: 245, radius: 75 },
	{ x: 585, y: 180, radius: 88 },
	{ x: 779, y: 106, radius: 100 },
	{ x: 1123, y: 215, radius: 82 },
	{ x: 1195, y: 150, radius: 90 },
	{ x: 1295, y: 298, radius: 78 },
	{ x: 1455, y: 260, radius: 80 },
	{ x: 170, y: 535, radius: 42 },
	{ x: 540, y: 422, radius: 42 },
	{ x: 1168, y: 454, radius: 42 },
	{ x: 510, y: 803, radius: 38 },
] as const

function createDeterministicRandom(seed: number): () => number {
	let state = seed >>> 0
	return () => {
		state = (Math.imul(state, 1664525) + 1013904223) >>> 0
		return state / 4294967296
	}
}

export function createLumeRainDropSpecs(seed = 0x4c554d45): LumeRainDropSpec[] {
	const random = createDeterministicRandom(seed)
	const drops: LumeRainDropSpec[] = []

	for (const pane of WINDOW_PANES) {
		for (let index = 0; index < 3; index++) {
			const length = 18 + random() * 16
			drops.push({
				x: pane.x + 8 + random() * Math.max(1, pane.width - 16),
				y: pane.y + random() * pane.height,
				minY: pane.y - length,
				maxY: pane.y + pane.height,
				length,
				speed: 26 + random() * 20,
				alpha: 0.1 + random() * 0.11,
				width: 0.8 + random() * 0.7,
			})
		}
	}

	return drops
}

function readSceneAmount(host: HTMLElement | null, property: string): number {
	if (host === null) {
		return 0
	}
	const value = Number.parseFloat(
		window.getComputedStyle(host).getPropertyValue(property),
	)
	if (!Number.isFinite(value)) {
		return 0
	}
	return Math.min(1, Math.max(0, value))
}

function approach(
	current: number,
	target: number,
	deltaSeconds: number,
): number {
	const progress = 1 - Math.exp(-2.6 * deltaSeconds)
	return current + (target - current) * progress
}

export function createLumeRainyScene(
	context: LivingSceneFactoryContext,
): LivingSceneInstance {
	const scene = context.makeContainer()
	const atmosphere = context.makeContainer()
	const lampGlow = context.makeContainer()
	const rain = context.makeContainer()
	scene.addChild(atmosphere)
	scene.addChild(lampGlow)
	scene.addChild(rain)
	context.root.addChild(scene)

	const nightVeil = context
		.makeGraphics()
		.rect(0, 0, DESIGN_WIDTH, DESIGN_HEIGHT)
		.fill({ color: 0x1c3e5f, alpha: 1 })
	nightVeil.alpha = 0
	atmosphere.addChild(nightVeil)

	const windowTint = context.makeGraphics()
	for (const pane of WINDOW_PANES) {
		windowTint
			.rect(pane.x, pane.y, pane.width, pane.height)
			.fill({ color: 0x275c88, alpha: 1 })
	}
	windowTint.alpha = 0
	atmosphere.addChild(windowTint)

	const sunsetVeil = context
		.makeGraphics()
		.rect(0, 0, DESIGN_WIDTH, DESIGN_HEIGHT)
		.fill({ color: 0xc77a45, alpha: 1 })
	sunsetVeil.alpha = 0
	atmosphere.addChild(sunsetVeil)

	const glowGraphic = context.makeGraphics()
	for (const lamp of LAMP_GLOWS) {
		glowGraphic
			.circle(lamp.x, lamp.y, lamp.radius * 1.55)
			.fill({ color: 0xffba64, alpha: 0.012 })
			.circle(lamp.x, lamp.y, lamp.radius * 1.15)
			.fill({ color: 0xffbd68, alpha: 0.018 })
			.circle(lamp.x, lamp.y, lamp.radius * 0.78)
			.fill({ color: 0xffc371, alpha: 0.028 })
			.circle(lamp.x, lamp.y, lamp.radius * 0.42)
			.fill({ color: 0xffca7d, alpha: 0.042 })
	}
	lampGlow.alpha = 0
	lampGlow.addChild(glowGraphic)

	const rainDrops: AnimatedRainDrop[] = createLumeRainDropSpecs().map(
		(spec) => {
			const graphic = context
				.makeGraphics()
				.moveTo(0, 0)
				.lineTo(-2.2, spec.length)
				.stroke({
					color: 0xdceff8,
					width: spec.width,
					alpha: spec.alpha,
				})
			graphic.x = spec.x
			graphic.y = spec.y
			rain.addChild(graphic)
			return { ...spec, graphic }
		},
	)
	rain.alpha = 0.28

	let targetNight = 0
	let targetSunset = 0
	let targetLamp = 0
	let currentNight = 0
	let currentSunset = 0
	let currentLamp = 0
	let stateRefreshElapsedMs = SCENE_STATE_REFRESH_MS

	const refreshSceneState = () => {
		const host = context.getHost()
		targetNight = readSceneAmount(host, '--scene-night-amount')
		targetSunset = readSceneAmount(host, '--scene-sunset-amount')
		targetLamp = readSceneAmount(host, '--scene-lamp-intensity')
	}

	const update = (ticker: { deltaMS: number }) => {
		const deltaMs = Math.min(100, Math.max(0, ticker.deltaMS))
		const deltaSeconds = deltaMs / 1000
		stateRefreshElapsedMs += deltaMs
		if (stateRefreshElapsedMs >= SCENE_STATE_REFRESH_MS) {
			stateRefreshElapsedMs %= SCENE_STATE_REFRESH_MS
			refreshSceneState()
		}

		currentNight = approach(currentNight, targetNight, deltaSeconds)
		currentSunset = approach(currentSunset, targetSunset, deltaSeconds)
		currentLamp = approach(currentLamp, targetLamp, deltaSeconds)

		nightVeil.alpha = currentNight * 0.18
		windowTint.alpha = currentNight * 0.075
		sunsetVeil.alpha = currentSunset * 0.055
		lampGlow.alpha = currentLamp * 0.9
		rain.alpha = 0.27 + currentNight * 0.08

		for (const drop of rainDrops) {
			drop.graphic.y += drop.speed * deltaSeconds
			if (drop.graphic.y > drop.maxY) {
				drop.graphic.y = drop.minY
			}
		}
	}

	context.ticker.add(update)

	return {
		resize: (width, height) => {
			scene.scale.set(width / DESIGN_WIDTH, height / DESIGN_HEIGHT)
		},
		destroy: () => {
			context.ticker.remove(update)
			context.root.removeChild(scene)
			scene.destroy({ children: true })
		},
	}
}
