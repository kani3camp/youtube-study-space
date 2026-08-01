import { findContinuousPartTypeClockRange, PartType } from './time-table'
import {
	getJapanTimeParts,
	getTextTone,
	getTextToneForTheme,
	getTimeTheme,
	getTimeThemeFromPartType,
	getTwilightProgress,
	parseDebugTextTone,
	parseDebugTimeTheme,
	resolveThemePresentation,
	resolveTimeTheme,
	shouldUseContrastBridge,
	type TextTone,
	type TimeTheme,
} from './time-theme'

describe('getTimeThemeFromPartType', () => {
	test.each<[string, TimeTheme]>([
		[PartType.EarlyMorning, 'dawn'],
		[PartType.Morning, 'dawn'],
		[PartType.BeforeNoon, 'day'],
		[PartType.Noon, 'day'],
		[PartType.AfterNoon1, 'day'],
		[PartType.AfterNoon2, 'sunset'],
		[PartType.Evening, 'twilight'],
		[PartType.Night1, 'night'],
		[PartType.Night2, 'night'],
		[PartType.MidNight1, 'midnight'],
		[PartType.MidNight2, 'midnight'],
	])('%s を %s に変換する', (partType, expected) => {
		expect(getTimeThemeFromPartType(partType)).toBe(expected)
	})

	test('未知の partType は day にフォールバックする', () => {
		expect(getTimeThemeFromPartType('unknown-part-type')).toBe('day')
	})
})

describe('getTimeTheme', () => {
	test('UTC上では前日でも日本時間の朝として判定する', () => {
		const date = new Date('2026-08-01T21:00:00Z')

		expect(getJapanTimeParts(date)).toEqual({
			year: 2026,
			month: 8,
			day: 2,
			hours: 6,
			minutes: 0,
			seconds: 0,
		})
		expect(getTimeTheme(date)).toBe('dawn')
	})

	test('UTC 09:00 を日本時間18:00の薄暮として判定する', () => {
		expect(getTimeTheme(new Date('2026-08-01T09:00:00Z'))).toBe('twilight')
	})

	test.each<[string, TimeTheme]>([
		['2026-08-01T15:24:00Z', 'night'], // JST 00:24
		['2026-08-01T15:25:00Z', 'midnight'], // JST 00:25
		['2026-08-01T19:54:00Z', 'midnight'], // JST 04:54
		['2026-08-01T19:55:00Z', 'dawn'], // JST 04:55
		['2026-08-02T00:14:00Z', 'dawn'], // JST 09:14
		['2026-08-02T00:15:00Z', 'day'], // JST 09:15
		['2026-08-02T06:14:00Z', 'day'], // JST 15:14
		['2026-08-02T06:15:00Z', 'sunset'], // JST 15:15
		['2026-08-02T08:39:00Z', 'sunset'], // JST 17:39
		['2026-08-02T08:40:00Z', 'twilight'], // JST 17:40
		['2026-08-02T10:54:00Z', 'twilight'], // JST 19:54
		['2026-08-02T10:55:00Z', 'night'], // JST 19:55
	])('%s を既存時間割の境界に従って判定する', (iso, expected) => {
		expect(getTimeTheme(new Date(iso))).toBe(expected)
	})
})

describe('twilight の文字トーン', () => {
	test.each<[TimeTheme, TextTone]>([
		['dawn', 'dark'],
		['day', 'dark'],
		['sunset', 'dark'],
		['night', 'light'],
		['midnight', 'light'],
	])('%s の通常トーンを %s と判定する', (theme, expected) => {
		expect(getTextToneForTheme(theme)).toBe(expected)
	})

	test('Evening全体を作業・休憩境界でリセットしない連続範囲として返す', () => {
		expect(findContinuousPartTypeClockRange(PartType.Evening, 18, 6)).toEqual({
			starts: { h: 17, m: 40 },
			ends: { h: 19, m: 55 },
			durationMinutes: 135,
		})
		expect(findContinuousPartTypeClockRange(PartType.Evening, 18, 35)).toEqual({
			starts: { h: 17, m: 40 },
			ends: { h: 19, m: 55 },
			durationMinutes: 135,
		})
	})

	test.each<[string, TextTone]>([
		['2026-08-01T08:40:00Z', 'dark'], // JST 17:40:00
		['2026-08-01T09:05:00Z', 'dark'], // JST 18:05:00 (休憩開始)
		['2026-08-01T09:35:00Z', 'dark'], // JST 18:35:00 (休憩開始)
		['2026-08-01T09:47:29Z', 'dark'], // JST 18:47:29
		['2026-08-01T09:47:30Z', 'light'], // JST 18:47:30 (135分の中点)
		['2026-08-01T10:54:59Z', 'light'], // JST 19:54:59
	])('%s のトーンを %s と判定する', (iso, expected) => {
		expect(getTextTone(new Date(iso))).toBe(expected)
	})

	test('進捗はEvening全体で単調増加し、中点が0.5になる', () => {
		const beforeBreak = getTwilightProgress(new Date('2026-08-01T09:04:59Z'))
		const atBreak = getTwilightProgress(new Date('2026-08-01T09:05:00Z'))
		const afterBreak = getTwilightProgress(new Date('2026-08-01T09:05:01Z'))

		expect(beforeBreak).toBeLessThan(atBreak as number)
		expect(atBreak).toBeLessThan(afterBreak as number)
		expect(getTwilightProgress(new Date('2026-08-01T09:47:30Z'))).toBe(0.5)
	})

	test('Eveningの外では進捗を返さない', () => {
		expect(
			getTwilightProgress(new Date('2026-08-01T08:39:59Z')),
		).toBeUndefined()
		expect(
			getTwilightProgress(new Date('2026-08-01T10:55:00Z')),
		).toBeUndefined()
	})

	test('深夜から早朝への日付またぎも日本時間基準で判定する', () => {
		expect(getTextTone(new Date('2026-08-01T19:54:59Z'))).toBe('light')
		expect(getTextTone(new Date('2026-08-01T19:55:00Z'))).toBe('dark')
	})
})

describe('debug query', () => {
	test.each<TimeTheme>([
		'dawn',
		'day',
		'sunset',
		'twilight',
		'night',
		'midnight',
	])('デバッグ時に %s を受け入れる', (theme) => {
		expect(parseDebugTimeTheme(theme, true)).toBe(theme)
	})

	test.each<TextTone>([
		'dark',
		'light',
	])('デバッグ時に文字トーン %s を受け入れる', (tone) => {
		expect(parseDebugTextTone(tone, true)).toBe(tone)
	})

	test('未知のテーマ名と文字トーンを拒否する', () => {
		expect(parseDebugTimeTheme('unknown', true)).toBeUndefined()
		expect(parseDebugTextTone('unknown', true)).toBeUndefined()
	})

	test('複数値のクエリを拒否する', () => {
		expect(parseDebugTimeTheme(['night', 'day'], true)).toBeUndefined()
		expect(parseDebugTextTone(['dark', 'light'], true)).toBeUndefined()
	})

	test('デバッグ無効時は有効な値も無視する', () => {
		expect(parseDebugTimeTheme('night', false)).toBeUndefined()
		expect(parseDebugTextTone('light', false)).toBeUndefined()
	})

	test('テーマと文字トーンを独立して固定できる', () => {
		expect(
			resolveThemePresentation(
				new Date('2026-08-01T21:00:00Z'),
				'twilight',
				'light',
				true,
			),
		).toEqual({ timeTheme: 'twilight', textTone: 'light' })
	})

	test('無効な固定値では日本時間による自動判定へ戻る', () => {
		expect(
			resolveTimeTheme(new Date('2026-08-01T21:00:00Z'), 'unknown', true),
		).toBe('dawn')
	})

	test('デバッグ無効時はクエリより日本時間判定を優先する', () => {
		expect(
			resolveTimeTheme(new Date('2026-08-01T21:00:00Z'), 'midnight', false),
		).toBe('dawn')
	})
})

describe('shouldUseContrastBridge', () => {
	test('同じ文字トーン間ではブリッジを使わない', () => {
		expect(
			shouldUseContrastBridge(
				{ timeTheme: 'day', textTone: 'dark' },
				{ timeTheme: 'sunset', textTone: 'dark' },
			),
		).toBe(false)
	})

	test('通常の明暗反転ではブリッジを使う', () => {
		expect(
			shouldUseContrastBridge(
				{ timeTheme: 'midnight', textTone: 'light' },
				{ timeTheme: 'dawn', textTone: 'dark' },
			),
		).toBe(true)
	})

	test('twilight中点の明暗反転は安全面上で即時に行う', () => {
		expect(
			shouldUseContrastBridge(
				{ timeTheme: 'twilight', textTone: 'dark' },
				{ timeTheme: 'twilight', textTone: 'light' },
			),
		).toBe(false)
	})
})
