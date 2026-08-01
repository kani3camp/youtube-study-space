import { PartType } from './time-table'
import {
	getJapanTimeParts,
	getTimeTheme,
	getTimeThemeFromPartType,
	parseDebugTimeTheme,
	resolveTimeTheme,
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
		[PartType.Evening, 'sunset'],
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
		})
		expect(getTimeTheme(date)).toBe('dawn')
	})

	test('UTC 09:00 を日本時間18:00の夕方として判定する', () => {
		expect(getTimeTheme(new Date('2026-08-01T09:00:00Z'))).toBe('sunset')
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
		['2026-08-02T10:54:00Z', 'sunset'], // JST 19:54
		['2026-08-02T10:55:00Z', 'night'], // JST 19:55
	])('%s を既存時間割の境界に従って判定する', (iso, expected) => {
		expect(getTimeTheme(new Date(iso))).toBe(expected)
	})
})

describe('parseDebugTimeTheme', () => {
	test.each<TimeTheme>([
		'dawn',
		'day',
		'sunset',
		'night',
		'midnight',
	])('デバッグ時に %s を受け入れる', (theme) => {
		expect(parseDebugTimeTheme(theme, true)).toBe(theme)
	})

	test('未知のテーマ名を拒否する', () => {
		expect(parseDebugTimeTheme('unknown', true)).toBeUndefined()
	})

	test('複数値のクエリを拒否する', () => {
		expect(parseDebugTimeTheme(['night', 'day'], true)).toBeUndefined()
	})

	test('デバッグ無効時は有効なテーマ名も無視する', () => {
		expect(parseDebugTimeTheme('night', false)).toBeUndefined()
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
