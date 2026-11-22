import type { Timestamp } from 'firebase/firestore'
import type { WorkNameTrend } from '../types/api'
import type { SystemConstants } from './firestore'
import {
	handleConstantsSnapshot,
	handleWorkNameTrendSnapshot,
} from './snapshot-handlers'

const constants: SystemConstants = {
	max_seats: 10,
	member_max_seats: 5,
	min_vacancy_rate: 0.2,
	youtube_membership_enabled: true,
	fixed_max_seats_enabled: false,
}

const trend = (): WorkNameTrend => ({
	ranking: [],
	ranked_at: {} as Timestamp,
})

describe('handleConstantsSnapshot', () => {
	test('reports a missing config/constants document as a read error without notifying constants', () => {
		const onConstants = vi.fn()
		const onReadError = vi.fn()

		handleConstantsSnapshot(
			{ exists: () => false, data: () => undefined },
			onConstants,
			onReadError,
		)

		expect(onConstants).not.toHaveBeenCalled()
		expect(onReadError).toHaveBeenCalledTimes(1)
		expect(onReadError.mock.calls[0][0]).toBeInstanceOf(Error)
	})

	test('notifies the constants of an existing document and reads data once', () => {
		const data = vi.fn(() => constants)
		const onConstants = vi.fn()
		const onReadError = vi.fn()

		handleConstantsSnapshot(
			{ exists: () => true, data },
			onConstants,
			onReadError,
		)

		expect(onConstants).toHaveBeenCalledWith(constants)
		expect(onReadError).not.toHaveBeenCalled()
		expect(data).toHaveBeenCalledTimes(1)
	})
})

describe('handleWorkNameTrendSnapshot', () => {
	test('routes multiple documents to the read error handler instead of throwing', () => {
		const onWorkNameTrend = vi.fn()
		const onReadError = vi.fn()

		expect(() =>
			handleWorkNameTrendSnapshot(
				{ docs: [{ data: trend }, { data: trend }] },
				onWorkNameTrend,
				onReadError,
			),
		).not.toThrow()

		expect(onWorkNameTrend).not.toHaveBeenCalled()
		expect(onReadError).toHaveBeenCalledTimes(1)
		expect(onReadError.mock.calls[0][0]).toBeInstanceOf(Error)
	})

	test('notifies the single document', () => {
		const value = trend()
		const onWorkNameTrend = vi.fn()
		const onReadError = vi.fn()

		handleWorkNameTrendSnapshot(
			{ docs: [{ data: () => value }] },
			onWorkNameTrend,
			onReadError,
		)

		expect(onWorkNameTrend).toHaveBeenCalledWith(value)
		expect(onReadError).not.toHaveBeenCalled()
	})

	test('keeps the previous value when the collection is empty', () => {
		const onWorkNameTrend = vi.fn()
		const onReadError = vi.fn()

		handleWorkNameTrendSnapshot({ docs: [] }, onWorkNameTrend, onReadError)

		expect(onWorkNameTrend).not.toHaveBeenCalled()
		expect(onReadError).not.toHaveBeenCalled()
	})
})
