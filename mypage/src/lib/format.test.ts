import { describe, expect, it } from 'vitest'

import { formatSeatId } from './format'

describe('formatSeatId', () => {
	it('formats general seat IDs as plain numbers', () => {
		expect(formatSeatId(12, false)).toBe('12')
	})

	it('formats member seat IDs with the VIP prefix', () => {
		expect(formatSeatId(12, true)).toBe('VIP12')
	})
})
