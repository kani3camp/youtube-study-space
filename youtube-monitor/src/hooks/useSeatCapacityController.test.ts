import { act, renderHook, waitFor } from '@testing-library/react'
import fetcher from '../lib/fetcher'
import type { SystemConstants } from '../lib/firestore'
import type { Seat } from '../types/api'

jest.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: jest.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: jest.fn(() => ({
		style: { fontFamily: 'mock-source-code-pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

jest.mock('../lib/fetcher', () => ({
	__esModule: true,
	default: jest.fn(),
}))

const mockedFetcher = jest.mocked(fetcher)

const originalNextPublicDebug = process.env.NEXT_PUBLIC_DEBUG
const originalNextPublicChannelGl = process.env.NEXT_PUBLIC_CHANNEL_GL
const originalNextPublicRoomConfig = process.env.NEXT_PUBLIC_ROOM_CONFIG

beforeAll(() => {
	process.env.NEXT_PUBLIC_DEBUG = 'true'
	process.env.NEXT_PUBLIC_CHANNEL_GL = 'false'
	process.env.NEXT_PUBLIC_ROOM_CONFIG = 'DEV'
})

afterAll(() => {
	if (originalNextPublicDebug === undefined) {
		delete process.env.NEXT_PUBLIC_DEBUG
	} else {
		process.env.NEXT_PUBLIC_DEBUG = originalNextPublicDebug
	}
	if (originalNextPublicChannelGl === undefined) {
		delete process.env.NEXT_PUBLIC_CHANNEL_GL
	} else {
		process.env.NEXT_PUBLIC_CHANNEL_GL = originalNextPublicChannelGl
	}
	if (originalNextPublicRoomConfig === undefined) {
		delete process.env.NEXT_PUBLIC_ROOM_CONFIG
	} else {
		process.env.NEXT_PUBLIC_ROOM_CONFIG = originalNextPublicRoomConfig
	}
})

function loadSeatCapacityController() {
	return (
		require('./useSeatCapacityController') as typeof import('./useSeatCapacityController')
	).default
}

const systemConstants: SystemConstants = {
	max_seats: 1,
	member_max_seats: 1,
	min_vacancy_rate: 0.5,
	youtube_membership_enabled: true,
	fixed_max_seats_enabled: false,
}

const baseProps = {
	enabled: true,
	ready: false,
	generalSeats: [] as Seat[],
	memberSeats: [] as Seat[],
	systemConstants,
}

describe('useSeatCapacityController', () => {
	beforeEach(() => {
		mockedFetcher.mockReset()
		mockedFetcher.mockResolvedValue({ result: 'ok', message: '' })
	})

	test('waits for the initial seat snapshots before sending a request', async () => {
		const useSeatCapacityController = loadSeatCapacityController()
		const { rerender } = renderHook(
			({ ready }: { ready: boolean }) =>
				useSeatCapacityController({ ...baseProps, ready }),
			{ initialProps: { ready: false } },
		)

		expect(mockedFetcher).not.toHaveBeenCalled()

		await act(async () => {
			rerender({ ready: true })
		})

		await waitFor(() => {
			expect(mockedFetcher).toHaveBeenCalledTimes(1)
		})
	})

	test('serializes requests and reviews the latest seat count afterwards', async () => {
		const useSeatCapacityController = loadSeatCapacityController()
		let resolveFirstRequest: (value: {
			result: string
			message: string
		}) => void = () => {}
		const firstRequest = new Promise<{ result: string; message: string }>(
			(resolve) => {
				resolveFirstRequest = resolve
			},
		)
		mockedFetcher.mockImplementationOnce(() => firstRequest)

		const { rerender } = renderHook(
			({ generalSeats }: { generalSeats: Seat[] }) =>
				useSeatCapacityController({
					...baseProps,
					ready: true,
					generalSeats,
				}),
			{ initialProps: { generalSeats: [] as Seat[] } },
		)

		await waitFor(() => {
			expect(mockedFetcher).toHaveBeenCalledTimes(1)
		})

		await act(async () => {
			rerender({ generalSeats: Array.from({ length: 13 }, () => ({}) as Seat) })
		})
		expect(mockedFetcher).toHaveBeenCalledTimes(1)

		await act(async () => {
			resolveFirstRequest({ result: 'ok', message: '' })
		})

		await waitFor(() => {
			expect(mockedFetcher).toHaveBeenCalledTimes(2)
		})
	})
})
