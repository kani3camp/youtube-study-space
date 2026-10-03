import { describe, expect, it } from 'vitest'

import { formatClockTime, formatSeatId } from './format'

describe('formatSeatId', () => {
	it('formats general seat IDs as plain numbers', () => {
		expect(formatSeatId(12, false)).toBe('12')
	})

	it('formats member seat IDs with the VIP prefix', () => {
		expect(formatSeatId(12, true)).toBe('VIP12')
	})
})

describe('formatClockTime', () => {
	it('formats an ISO date as a 24-hour JST time', () => {
		const value = '2026-08-14T10:05:00.000Z'
		expect(formatClockTime(value)).toBe('19:05')
	})

	it('returns a placeholder for invalid input', () => {
		expect(formatClockTime('invalid')).toBe('--:--')
	})
})
