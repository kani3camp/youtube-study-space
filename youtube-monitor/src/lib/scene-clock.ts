import { getJapanTimeParts } from './time-theme'

export const SCENE_DAY_SECONDS = 24 * 60 * 60
export const MAX_DEBUG_SCENE_SPEED = 1440

export type SceneState = {
	brightness: number
	saturation: number
	warmth: number
	sunsetAmount: number
	nightAmount: number
	lampIntensity: number
	skyBrightness: number
}

type SceneKeyframe = SceneState & {
	atSeconds: number
}

export type SceneClockAnchor = {
	realStartedAtMs: number
	sceneStartedAtSeconds: number
	speed: number
}

const minutes = (hours: number, mins: number) => (hours * 60 + mins) * 60

const SCENE_KEYFRAMES: readonly SceneKeyframe[] = [
	{
		atSeconds: 0,
		brightness: 0.78,
		saturation: 0.88,
		warmth: 0.08,
		sunsetAmount: 0,
		nightAmount: 0.82,
		lampIntensity: 0.95,
		skyBrightness: 0.2,
	},
	{
		atSeconds: minutes(0, 25),
		brightness: 0.72,
		saturation: 0.82,
		warmth: 0.04,
		sunsetAmount: 0,
		nightAmount: 1,
		lampIntensity: 1,
		skyBrightness: 0.12,
	},
	{
		atSeconds: minutes(4, 55),
		brightness: 0.84,
		saturation: 0.9,
		warmth: 0.38,
		sunsetAmount: 0.06,
		nightAmount: 0.5,
		lampIntensity: 0.72,
		skyBrightness: 0.45,
	},
	{
		atSeconds: minutes(9, 15),
		brightness: 1,
		saturation: 1,
		warmth: 0.16,
		sunsetAmount: 0,
		nightAmount: 0,
		lampIntensity: 0,
		skyBrightness: 1,
	},
	{
		atSeconds: minutes(15, 15),
		brightness: 0.98,
		saturation: 1.02,
		warmth: 0.36,
		sunsetAmount: 0.18,
		nightAmount: 0,
		lampIntensity: 0.04,
		skyBrightness: 0.94,
	},
	{
		atSeconds: minutes(17, 40),
		brightness: 0.9,
		saturation: 1.04,
		warmth: 1,
		sunsetAmount: 1,
		nightAmount: 0.12,
		lampIntensity: 0.28,
		skyBrightness: 0.68,
	},
	{
		atSeconds: minutes(18, 47),
		brightness: 0.83,
		saturation: 0.96,
		warmth: 0.5,
		sunsetAmount: 0.48,
		nightAmount: 0.5,
		lampIntensity: 0.7,
		skyBrightness: 0.42,
	},
	{
		atSeconds: minutes(19, 55),
		brightness: 0.78,
		saturation: 0.88,
		warmth: 0.12,
		sunsetAmount: 0.04,
		nightAmount: 0.88,
		lampIntensity: 1,
		skyBrightness: 0.2,
	},
	{
		atSeconds: SCENE_DAY_SECONDS,
		brightness: 0.78,
		saturation: 0.88,
		warmth: 0.08,
		sunsetAmount: 0,
		nightAmount: 0.82,
		lampIntensity: 0.95,
		skyBrightness: 0.2,
	},
]

const clamp01 = (value: number) => Math.min(1, Math.max(0, value))
const lerp = (from: number, to: number, progress: number) =>
	from + (to - from) * progress

export function normalizeSceneClockSeconds(seconds: number): number {
	return ((seconds % SCENE_DAY_SECONDS) + SCENE_DAY_SECONDS) % SCENE_DAY_SECONDS
}

export function getJapanClockSeconds(date: Date): number {
	const { hours, minutes, seconds } = getJapanTimeParts(date)
	return hours * 60 * 60 + minutes * 60 + seconds
}

export function getSceneStateAtClockSeconds(clockSeconds: number): SceneState {
	const normalized = normalizeSceneClockSeconds(clockSeconds)
	const nextIndex = SCENE_KEYFRAMES.findIndex(
		(keyframe) => keyframe.atSeconds > normalized,
	)
	const right = SCENE_KEYFRAMES[nextIndex]
	const left = SCENE_KEYFRAMES[Math.max(0, nextIndex - 1)]
	const duration = right.atSeconds - left.atSeconds
	const progress = duration === 0 ? 0 : (normalized - left.atSeconds) / duration

	return {
		brightness: lerp(left.brightness, right.brightness, progress),
		saturation: lerp(left.saturation, right.saturation, progress),
		warmth: lerp(left.warmth, right.warmth, progress),
		sunsetAmount: lerp(left.sunsetAmount, right.sunsetAmount, progress),
		nightAmount: lerp(left.nightAmount, right.nightAmount, progress),
		lampIntensity: lerp(left.lampIntensity, right.lampIntensity, progress),
		skyBrightness: lerp(left.skyBrightness, right.skyBrightness, progress),
	}
}

export function parseDebugSceneTime(
	value: string | string[] | undefined,
	debugEnabled: boolean,
): number | undefined {
	if (!debugEnabled || typeof value !== 'string') {
		return undefined
	}
	const match = /^(\\d{2}):(\\d{2})$/.exec(value)
	if (!match) {
		return undefined
	}
	const hours = Number(match[1])
	const mins = Number(match[2])
	if (hours < 0 || hours > 23 || mins < 0 || mins > 59) {
		return undefined
	}
	return minutes(hours, mins)
}

export function parseDebugSceneSpeed(
	value: string | string[] | undefined,
	debugEnabled: boolean,
): number | undefined {
	if (!debugEnabled || typeof value !== 'string' || value.trim() === '') {
		return undefined
	}
	const parsed = Number(value)
	if (!Number.isFinite(parsed) || parsed < 0 || parsed > MAX_DEBUG_SCENE_SPEED) {
		return undefined
	}
	return parsed
}

export function createSceneClockAnchor(
	now: Date,
	sceneTimeQuery: string | string[] | undefined,
	sceneSpeedQuery: string | string[] | undefined,
	debugEnabled: boolean,
): SceneClockAnchor {
	const debugTime = parseDebugSceneTime(sceneTimeQuery, debugEnabled)
	const debugSpeed = parseDebugSceneSpeed(sceneSpeedQuery, debugEnabled)
	return {
		realStartedAtMs: now.getTime(),
		sceneStartedAtSeconds: debugTime ?? getJapanClockSeconds(now),
		speed: debugSpeed ?? (debugTime === undefined ? 1 : 0),
	}
}

export function getSceneClockSecondsAt(
	anchor: SceneClockAnchor,
	realNowMs: number,
): number {
	const elapsedRealSeconds = (realNowMs - anchor.realStartedAtMs) / 1000
	return normalizeSceneClockSeconds(
		anchor.sceneStartedAtSeconds + elapsedRealSeconds * anchor.speed,
	)
}

const compactNumber = (value: number) => Number(value.toFixed(4))

export function buildAmbientSceneFilter(state: SceneState): string {
	const sepia = clamp01(state.warmth * 0.14 + state.nightAmount * 0.025)
	const hueRotateDeg = state.nightAmount * 8 - state.sunsetAmount * 5
	const contrast = 1 - state.nightAmount * 0.04
	return [
		`brightness(${compactNumber(state.brightness)})`,
		`saturate(${compactNumber(state.saturation)})`,
		`sepia(${compactNumber(sepia)})`,
		`hue-rotate(${compactNumber(hueRotateDeg)}deg)`,
		`contrast(${compactNumber(contrast)})`,
	].join(' ')
}
