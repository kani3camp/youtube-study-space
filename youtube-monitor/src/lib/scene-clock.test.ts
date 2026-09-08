import {
	buildAmbientSceneFilter,
	createSceneClockAnchor,
	getJapanClockSeconds,
	getSceneClockSecondsAt,
	getSceneStateAtClockSeconds,
	parseDebugSceneSpeed,
	parseDebugSceneTime,
	SCENE_DAY_SECONDS,
} from './scene-clock'

describe('Scene Clock', () => {
	test('日本時間の壁時計秒へ変換する', () => {
		expect(getJapanClockSeconds(new Date('2026-08-01T21:00:15Z'))).toBe(
			6 * 60 * 60 + 15,
		)
	})

	test('day keyframeは昼の明るさを返す', () => {
		const state = getSceneStateAtClockSeconds((9 * 60 + 15) * 60)
		expect(state.brightness).toBe(1)
		expect(state.nightAmount).toBe(0)
		expect(state.lampIntensity).toBe(0)
	})

	test('keyframe間を連続補間する', () => {
		const left = getSceneStateAtClockSeconds((15 * 60 + 15) * 60)
		const middle = getSceneStateAtClockSeconds((16 * 60 + 27.5) * 60)
		const right = getSceneStateAtClockSeconds((17 * 60 + 40) * 60)

		expect(middle.brightness).toBeLessThan(left.brightness)
		expect(middle.brightness).toBeGreaterThan(right.brightness)
		expect(middle.sunsetAmount).toBeGreaterThan(left.sunsetAmount)
		expect(middle.sunsetAmount).toBeLessThan(right.sunsetAmount)
	})

	test('24時を越えて循環する', () => {
		expect(getSceneStateAtClockSeconds(SCENE_DAY_SECONDS)).toEqual(
			getSceneStateAtClockSeconds(0),
		)
		expect(getSceneStateAtClockSeconds(-1)).toEqual(
			getSceneStateAtClockSeconds(SCENE_DAY_SECONDS - 1),
		)
	})
})

describe('Scene Clock debug query', () => {
	test('sceneTime HH:mmを受け入れる', () => {
		expect(parseDebugSceneTime('17:30', true)).toBe((17 * 60 + 30) * 60)
	})

	test.each(['24:00', '12:60', '7:30', 'unknown'])(
		'不正なsceneTime %sを拒否する',
		(value) => {
			expect(parseDebugSceneTime(value, true)).toBeUndefined()
		},
	)

	test('sceneSpeedは0〜1440倍を受け入れる', () => {
		expect(parseDebugSceneSpeed('0', true)).toBe(0)
		expect(parseDebugSceneSpeed('720', true)).toBe(720)
		expect(parseDebugSceneSpeed('1440', true)).toBe(1440)
	})

	test('デバッグ無効時はqueryを無視する', () => {
		expect(parseDebugSceneTime('17:30', false)).toBeUndefined()
		expect(parseDebugSceneSpeed('720', false)).toBeUndefined()
	})

	test('sceneTimeだけなら固定し、sceneSpeed併用なら加速する', () => {
		const now = new Date('2026-08-01T00:00:00Z')
		const frozen = createSceneClockAnchor(now, '17:30', undefined, true)
		const accelerated = createSceneClockAnchor(now, '17:30', '720', true)

		expect(getSceneClockSecondsAt(frozen, now.getTime() + 60_000)).toBe(
			(17 * 60 + 30) * 60,
		)
		expect(getSceneClockSecondsAt(accelerated, now.getTime() + 60_000)).toBe(
			(5 * 60 + 30) * 60,
		)
	})
})

describe('Ambient filter', () => {
	test('SceneStateをCSS filterへ変換する', () => {
		const filter = buildAmbientSceneFilter(
			getSceneStateAtClockSeconds((17 * 60 + 40) * 60),
		)
		expect(filter).toContain('brightness(')
		expect(filter).toContain('saturate(')
		expect(filter).toContain('sepia(')
		expect(filter).toContain('hue-rotate(')
		expect(filter).toContain('contrast(')
	})
})
