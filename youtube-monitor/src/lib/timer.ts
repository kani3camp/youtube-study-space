import {
	getCurrentSection,
	getNextSection,
	getSectionDateRange,
	SectionType,
} from './time-table'

export type RemainingInfo = {
	remainingSec: number
	percentage: number
	isStudy: boolean
	nextLabel: string
	nextDurationMin: number
}

export const FALLBACK_REMAINING: RemainingInfo = {
	remainingSec: 0,
	percentage: 0,
	isStudy: false,
	nextLabel: '',
	nextDurationMin: 0,
}

export function computeRemaining(now: Date): RemainingInfo {
	const section = getCurrentSection(now)
	const { startsAt, endsAt } = getSectionDateRange(section, now)
	const sectionDurationSec = Math.max(
		1,
		Math.floor((endsAt.getTime() - startsAt.getTime()) / 1000),
	)
	const remainingSec = Math.max(
		0,
		Math.ceil((endsAt.getTime() - now.getTime()) / 1000),
	)
	const percentage = (remainingSec / sectionDurationSec) * 100
	const isStudy = section.sectionType === SectionType.Study
	const next = getNextSection(now)
	const nextRange = getSectionDateRange(next, endsAt)
	const nextDurationMin = Math.floor(
		(nextRange.endsAt.getTime() - nextRange.startsAt.getTime()) / 60000,
	)
	const nextLabel = next.sectionType === SectionType.Study ? 'study' : 'break'
	return {
		remainingSec,
		percentage,
		isStudy,
		nextLabel,
		nextDurationMin,
	}
}

export function formatRemainingTime(remainingSec: number): {
	minutes: string
	seconds: string
} {
	return {
		minutes: String(Math.floor(remainingSec / 60)),
		seconds: String(remainingSec % 60).padStart(2, '0'),
	}
}
