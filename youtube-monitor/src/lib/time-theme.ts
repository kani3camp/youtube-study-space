import {
	findContinuousPartTypeClockRange,
	findSectionAtClockTime,
	PartType,
} from './time-table'

export const TIME_THEMES = [
	'dawn',
	'day',
	'sunset',
	'twilight',
	'night',
	'midnight',
] as const

export type TimeTheme = (typeof TIME_THEMES)[number]

export const TEXT_TONES = ['dark', 'light'] as const
export type TextTone = (typeof TEXT_TONES)[number]

export type ThemePresentation = {
	timeTheme: TimeTheme
	textTone: TextTone
}

export type JapanTimeParts = {
	year: number
	month: number
	day: number
	hours: number
	minutes: number
	seconds: number
}

export const TWILIGHT_TONE_SWITCH_PROGRESS = 0.5

const PART_TYPE_TO_THEME: Readonly<Record<string, TimeTheme>> = {
	[PartType.EarlyMorning]: 'dawn',
	[PartType.Morning]: 'dawn',
	[PartType.BeforeNoon]: 'day',
	[PartType.Noon]: 'day',
	[PartType.AfterNoon1]: 'day',
	[PartType.AfterNoon2]: 'sunset',
	[PartType.Evening]: 'twilight',
	[PartType.Night1]: 'night',
	[PartType.Night2]: 'night',
	[PartType.MidNight1]: 'midnight',
	[PartType.MidNight2]: 'midnight',
}

const STATIC_THEME_TEXT_TONES: Readonly<
	Record<Exclude<TimeTheme, 'twilight'>, TextTone>
> = {
	dawn: 'dark',
	day: 'dark',
	sunset: 'dark',
	night: 'light',
	midnight: 'light',
}

const japanTimeFormatter = new Intl.DateTimeFormat('en-GB', {
	timeZone: 'Asia/Tokyo',
	year: 'numeric',
	month: '2-digit',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit',
	second: '2-digit',
	hourCycle: 'h23',
})

export function getTimeThemeFromPartType(partType: string): TimeTheme {
	return PART_TYPE_TO_THEME[partType] ?? 'day'
}

export function getJapanTimeParts(date: Date): JapanTimeParts {
	const parts = Object.fromEntries(
		japanTimeFormatter
			.formatToParts(date)
			.filter((part) => part.type !== 'literal')
			.map((part) => [part.type, Number(part.value)]),
	)

	return {
		year: parts.year,
		month: parts.month,
		day: parts.day,
		hours: parts.hour,
		minutes: parts.minute,
		seconds: parts.second,
	}
}

export function getTimeTheme(date: Date): TimeTheme {
	const { hours, minutes } = getJapanTimeParts(date)
	const section = findSectionAtClockTime(hours, minutes)
	return section ? getTimeThemeFromPartType(section.partType) : 'day'
}

export function getTwilightProgress(date: Date): number | undefined {
	const { hours, minutes, seconds } = getJapanTimeParts(date)
	const range = findContinuousPartTypeClockRange(
		PartType.Evening,
		hours,
		minutes,
	)
	if (!range) {
		return undefined
	}

	const startsAt = range.starts.h * 60 + range.starts.m
	const currentMinute = hours * 60 + minutes + seconds / 60
	const elapsedMinutes = (currentMinute - startsAt + 24 * 60) % (24 * 60)
	return Math.min(1, Math.max(0, elapsedMinutes / range.durationMinutes))
}

export function getTextToneForTheme(
	timeTheme: TimeTheme,
	twilightProgress = 0,
): TextTone {
	if (timeTheme === 'twilight') {
		return twilightProgress < TWILIGHT_TONE_SWITCH_PROGRESS ? 'dark' : 'light'
	}
	return STATIC_THEME_TEXT_TONES[timeTheme]
}

export function getTextTone(date: Date): TextTone {
	const timeTheme = getTimeTheme(date)
	return getTextToneForTheme(timeTheme, getTwilightProgress(date) ?? 0)
}

export function parseDebugTimeTheme(
	queryValue: string | string[] | undefined,
	debugEnabled: boolean,
): TimeTheme | undefined {
	if (!debugEnabled || typeof queryValue !== 'string') {
		return undefined
	}

	return TIME_THEMES.find((theme) => theme === queryValue)
}

export function parseDebugTextTone(
	queryValue: string | string[] | undefined,
	debugEnabled: boolean,
): TextTone | undefined {
	if (!debugEnabled || typeof queryValue !== 'string') {
		return undefined
	}
	return TEXT_TONES.find((tone) => tone === queryValue)
}

export function resolveThemePresentation(
	date: Date,
	timeThemeQuery: string | string[] | undefined,
	textToneQuery: string | string[] | undefined,
	debugEnabled: boolean,
): ThemePresentation {
	const timeTheme =
		parseDebugTimeTheme(timeThemeQuery, debugEnabled) ?? getTimeTheme(date)
	const textTone =
		parseDebugTextTone(textToneQuery, debugEnabled) ??
		getTextToneForTheme(timeTheme, getTwilightProgress(date) ?? 0)
	return { timeTheme, textTone }
}

export function resolveTimeTheme(
	date: Date,
	queryValue: string | string[] | undefined,
	debugEnabled: boolean,
): TimeTheme {
	return resolveThemePresentation(date, queryValue, undefined, debugEnabled)
		.timeTheme
}

export function shouldUseContrastBridge(
	current: ThemePresentation,
	target: ThemePresentation,
): boolean {
	// 文字極性が変わる場合は、同一テーマ内でも必ず安全な中間面を経由します。
	return current.textTone !== target.textTone
}
