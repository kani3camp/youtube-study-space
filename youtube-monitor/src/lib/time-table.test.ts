import {
	getCurrentSection,
	getNextSection,
	getSectionDateRange,
	SectionType,
	type TimeSection,
} from './time-table'

const createDate = (hours: number, minutes: number, seconds = 0) =>
	new Date(2026, 2, 7, hours, minutes, seconds)

const crossMidnightStudySection: TimeSection = {
	starts: { h: 23, m: 40 },
	ends: { h: 0, m: 5 },
	sectionType: SectionType.Study,
	sectionId: 30,
	partType: 'common:part_type.night2',
}

describe('getCurrentSection', () => {
	test('通常の日中作業セクションを返す', () => {
		const section = getCurrentSection(createDate(7, 10))

		expect(section.sectionType).toBe(SectionType.Study)
		expect(section.sectionId).toBe(1)
		expect(section.starts).toEqual({ h: 7, m: 0 })
		expect(section.ends).toEqual({ h: 7, m: 25 })
	})

	test('通常の日中休憩セクションを返す', () => {
		const section = getCurrentSection(createDate(7, 27))

		expect(section.sectionType).toBe(SectionType.Break)
		expect(section.starts).toEqual({ h: 7, m: 25 })
		expect(section.ends).toEqual({ h: 7, m: 30 })
	})

	test('日付またぎの作業セクションを返す', () => {
		const section = getCurrentSection(createDate(23, 50))

		expect(section.sectionType).toBe(SectionType.Study)
		expect(section.sectionId).toBe(30)
		expect(section.starts).toEqual({ h: 23, m: 40 })
		expect(section.ends).toEqual({ h: 0, m: 5 })
	})

	test('日付またぎ後の休憩セクションを返す', () => {
		const section = getCurrentSection(createDate(0, 10))

		expect(section.sectionType).toBe(SectionType.Break)
		expect(section.starts).toEqual({ h: 0, m: 5 })
		expect(section.ends).toEqual({ h: 0, m: 25 })
	})

	test('境界時刻で次のセクションに切り替わる', () => {
		const section = getCurrentSection(createDate(7, 25))

		expect(section.sectionType).toBe(SectionType.Break)
		expect(section.starts).toEqual({ h: 7, m: 25 })
		expect(section.ends).toEqual({ h: 7, m: 30 })
	})

	test('引数のDateオブジェクトを破壊しない', () => {
		const now = createDate(23, 50)
		const timestamp = now.getTime()

		getCurrentSection(now)

		expect(now.getTime()).toBe(timestamp)
	})
})

describe('getNextSection', () => {
	test('日中の作業セクションの次に休憩セクションを返す', () => {
		const nextSection = getNextSection(createDate(7, 10))

		expect(nextSection.sectionType).toBe(SectionType.Break)
		expect(nextSection.starts).toEqual({ h: 7, m: 25 })
		expect(nextSection.ends).toEqual({ h: 7, m: 30 })
	})

	test('日付またぎでも次のセクションを正しく返す', () => {
		const nextSection = getNextSection(createDate(23, 50))

		expect(nextSection.sectionType).toBe(SectionType.Break)
		expect(nextSection.starts).toEqual({ h: 0, m: 5 })
		expect(nextSection.ends).toEqual({ h: 0, m: 25 })
	})

	test('休憩セクションの次に作業セクションを返す', () => {
		const nextSection = getNextSection(createDate(7, 27))

		expect(nextSection.sectionType).toBe(SectionType.Study)
		expect(nextSection.starts).toEqual({ h: 7, m: 30 })
		expect(nextSection.ends).toEqual({ h: 7, m: 55 })
	})
})

describe('getSectionDateRange', () => {
	test('通常セクションでは当日開始・当日終了になる', () => {
		const now = createDate(7, 10)
		const section = getCurrentSection(now)
		const { startsAt, endsAt } = getSectionDateRange(section, now)

		expect(startsAt).toEqual(createDate(7, 0))
		expect(endsAt).toEqual(createDate(7, 25))
	})

	test('日付またぎセクション中は当日開始・翌日終了になる', () => {
		const now = createDate(23, 50)
		const { startsAt, endsAt } = getSectionDateRange(
			crossMidnightStudySection,
			now,
		)

		expect(startsAt).toEqual(createDate(23, 40))
		expect(endsAt).toEqual(new Date(2026, 2, 8, 0, 5, 0))
	})

	test('日付またぎセクション開始前は前日開始・当日終了になる', () => {
		const now = createDate(22, 0)
		const { startsAt, endsAt } = getSectionDateRange(
			crossMidnightStudySection,
			now,
		)

		expect(startsAt).toEqual(new Date(2026, 2, 6, 23, 40, 0))
		expect(endsAt).toEqual(createDate(0, 5))
	})
})
