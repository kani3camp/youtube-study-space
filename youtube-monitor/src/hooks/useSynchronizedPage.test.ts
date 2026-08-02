import { getSynchronizedPageIndex } from './useSynchronizedPage'

const intervalMs = 8_000

describe('getSynchronizedPageIndex', () => {
	test('returns zero when there are no pages', () => {
		expect(getSynchronizedPageIndex(123_456, 0, intervalMs)).toBe(0)
	})

	test('returns zero when there is one page', () => {
		expect(getSynchronizedPageIndex(123_456, 1, intervalMs)).toBe(0)
	})

	test('uses the absolute time to select among multiple pages', () => {
		expect(getSynchronizedPageIndex(0, 3, intervalMs)).toBe(0)
		expect(getSynchronizedPageIndex(intervalMs, 3, intervalMs)).toBe(1)
		expect(getSynchronizedPageIndex(intervalMs * 2, 3, intervalMs)).toBe(2)
		expect(getSynchronizedPageIndex(intervalMs * 3, 3, intervalMs)).toBe(0)
	})

	test('handles the page boundary immediately before, at, and after it', () => {
		expect(getSynchronizedPageIndex(intervalMs - 1, 3, intervalMs)).toBe(0)
		expect(getSynchronizedPageIndex(intervalMs, 3, intervalMs)).toBe(1)
		expect(getSynchronizedPageIndex(intervalMs + 1, 3, intervalMs)).toBe(1)
	})

	test('does not depend on when either browser source was started', () => {
		const now = 2_345_678
		expect(getSynchronizedPageIndex(now, 5, intervalMs)).toBe(
			getSynchronizedPageIndex(now, 5, intervalMs),
		)
	})

	test('keeps the index in range when the page count changes', () => {
		for (const pageCount of [1, 2, 3, 8]) {
			const pageIndex = getSynchronizedPageIndex(987_654, pageCount, intervalMs)
			expect(pageIndex).toBeGreaterThanOrEqual(0)
			expect(pageIndex).toBeLessThan(pageCount)
		}
	})
})
