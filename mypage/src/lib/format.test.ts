import { describe, expect, it } from 'vitest'
import { formatWorkSeconds } from './format'

describe('work time display', () => {
	it('floors only after summing seconds', () => {
		expect(formatWorkSeconds(31 + 31)).toBe('0時間 1分')
		expect(formatWorkSeconds(3659)).toBe('1時間 0分')
	})
	it('distinguishes a successful zero from unavailable data', () => {
		expect(formatWorkSeconds(0)).toBe('0時間 0分')
		expect(formatWorkSeconds(null)).toBe('取得できません')
	})
})
