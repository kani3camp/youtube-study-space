import { findSectionAtClockTime, PartType } from './time-table'

export const TIME_THEMES = [
	'dawn',
	'day',
	'sunset',
	'night',
	'midnight',
] as const

export type TimeTheme = (typeof TIME_THEMES)[number]

export type JapanTimeParts = {
	year: number
	month: number
	day: number
	hours: number
	minutes: number
}

const PART_TYPE_TO_THEME: Readonly<Record<string, TimeTheme>> = {
	[PartType.EarlyMorning]: 'dawn',
	[PartType.Morning]: 'dawn',
	[PartType.BeforeNoon]: 'day',
	[PartType.Noon]: 'day',
	[PartType.AfterNoon1]: 'day',
	[PartType.AfterNoon2]: 'sunset',
	[PartType.Evening]: 'sunset',
	[PartType.Night1]: 'night',
	[PartType.Night2]: 'night',
	[PartType.MidNight1]: 'midnight',
	[PartType.MidNight2]: 'midnight',
}

const japanTimeFormatter = new Intl.DateTimeFormat('en-GB', {
	timeZone: 'Asia/Tokyo',
	year: 'numeric',
	month: '2-digit',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit',
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
	}
}

export function getTimeTheme(date: Date): TimeTheme {
	const { hours, minutes } = getJapanTimeParts(date)
	const section = findSectionAtClockTime(hours, minutes)
	return section ? getTimeThemeFromPartType(section.partType) : 'day'
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

export function resolveTimeTheme(
	date: Date,
	queryValue: string | string[] | undefined,
	debugEnabled: boolean,
): TimeTheme {
	return parseDebugTimeTheme(queryValue, debugEnabled) ?? getTimeTheme(date)
}
