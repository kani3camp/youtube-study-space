import { computeRemaining, formatRemainingTime } from './timer'

const createDate = (
	hours: number,
	minutes: number,
	seconds = 0,
	milliseconds = 0,
) => new Date(2026, 2, 7, hours, minutes, seconds, milliseconds)

describe('computeRemaining', () => {
	test('日中の作業セクションで残り時間と次セクションを正しく計算する', () => {
		const remaining = computeRemaining(createDate(7, 10))

		expect(remaining).toMatchObject({
			remainingSec: 15 * 60,
			percentage: 60,
			isStudy: true,
			nextLabel: 'break',
			nextDurationMin: 5,
		})
	})

	test('ミリ秒を含んでも秒表示が早く減らない', () => {
		const remaining = computeRemaining(createDate(7, 10, 0, 900))

		expect(remaining.remainingSec).toBe(15 * 60)
	})

	test('ミリ秒を含む場合はプログレス計算が秒単位に丸められない', () => {
		const remaining = computeRemaining(createDate(7, 10, 0, 900))

		expect(remaining.percentage).toBeCloseTo(59.94, 2)
	})

	test('日中の休憩セクションで残り時間と次セクションを正しく計算する', () => {
		const remaining = computeRemaining(createDate(7, 27))

		expect(remaining).toMatchObject({
			remainingSec: 3 * 60,
			percentage: 60,
			isStudy: false,
			nextLabel: 'study',
			nextDurationMin: 25,
		})
	})

	test('日付またぎの作業セクションで残り時間を正しく計算する', () => {
		const remaining = computeRemaining(createDate(23, 50))

		expect(remaining).toMatchObject({
			remainingSec: 15 * 60,
			percentage: 60,
			isStudy: true,
			nextLabel: 'break',
			nextDurationMin: 20,
		})
	})

	test('日付またぎ後の休憩セクションで残り時間を正しく計算する', () => {
		const remaining = computeRemaining(createDate(0, 10))

		expect(remaining).toMatchObject({
			remainingSec: 15 * 60,
			percentage: 75,
			isStudy: false,
			nextLabel: 'study',
			nextDurationMin: 25,
		})
	})
})

describe('formatRemainingTime', () => {
	test('分が1桁でもゼロ埋めしない', () => {
		expect(formatRemainingTime(7 * 60 + 14)).toEqual({
			minutes: '7',
			seconds: '14',
		})
	})

	test('0分でも表示しつつ秒は2桁で表示する', () => {
		expect(formatRemainingTime(46)).toEqual({
			minutes: '0',
			seconds: '46',
		})
	})

	test('2桁の分はそのまま維持し秒0をゼロ埋めする', () => {
		expect(formatRemainingTime(25 * 60)).toEqual({
			minutes: '25',
			seconds: '00',
		})
	})
})
