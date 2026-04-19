import { render, screen, waitFor } from '@testing-library/react'
import type { Timestamp } from 'firebase/firestore'
import type { ComponentPropsWithoutRef } from 'react'
import { act } from 'react'
import type { SeatProps } from './SeatBox'

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

jest.mock('next/image', () => ({
	__esModule: true,
	default: (props: ComponentPropsWithoutRef<'img'>) => {
		const { alt, src, ...rest } = props
		return (
			<span
				role="img"
				aria-label={alt}
				data-next-image=""
				data-src={typeof src === 'string' ? src : ''}
				{...rest}
			/>
		)
	},
}))

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

function loadSeatBox() {
	const seatBoxModule = require('./SeatBox') as typeof import('./SeatBox')
	seatBoxModule.resetMeasureTextContextForTest()
	return seatBoxModule.default
}

const GENERAL_SEAT_FONT_SIZE = 22.8
const GENERAL_SEAT_WIDTH_PX = 140
const GENERAL_SEAT_LINE_WIDTH_PX = 115.2

function createDeferredPromise<T>() {
	let resolve: (value: T | PromiseLike<T>) => void = () => {}
	const promise = new Promise<T>((innerResolve) => {
		resolve = innerResolve
	})
	return { promise, resolve }
}

function expectedFontSizePx({
	text,
	seatFontSizePx,
	lineWidthPx,
	measuredWidthPx,
	baseEm,
	minEm,
}: {
	text: string
	seatFontSizePx: number
	lineWidthPx: number
	measuredWidthPx: number
	baseEm: number
	minEm: number
}) {
	let fontSizePx = seatFontSizePx * baseEm
	if (text === '') {
		return fontSizePx
	}
	if (measuredWidthPx > lineWidthPx) {
		fontSizePx *= lineWidthPx / measuredWidthPx
		fontSizePx *= 0.95
		if (fontSizePx < seatFontSizePx * minEm) {
			fontSizePx = seatFontSizePx * minEm
		}
	}
	return fontSizePx
}

function fontSizePxOf(element: HTMLElement) {
	return Number.parseFloat(element.style.fontSize)
}

function createBaseProps(overrides: Partial<SeatProps> = {}): SeatProps {
	const timestamp = {} as Timestamp
	return {
		globalSeatId: 123,
		isUsed: true,
		memberOnly: false,
		hoursRemaining: 0,
		minutesRemaining: 10,
		hoursElapsed: 1,
		minutesElapsed: 3,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 123,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容',
			break_work_name: '',
			entered_at: timestamp,
			until: timestamp,
			appearance: {
				color_code1: '#5BD27D',
				color_code2: '#008CFF',
				num_stars: 0,
				color_gradient_enabled: false,
			},
			menu_code: '',
			state: 'work',
			current_state_started_at: timestamp,
			current_state_until: timestamp,
			cumulative_work_sec: 0,
			daily_cumulative_work_sec: 0,
			user_profile_image_url: '',
		},
		seatPosition: {
			x: 0,
			y: 0,
			rotate: 0,
		},
		seatShape: {
			widthPx: GENERAL_SEAT_WIDTH_PX,
			heightPx: 100,
		},
		roomShape: {
			widthPx: 1520,
			heightPx: 1000,
		},
		menuImageMap: new Map<string, string>(),
		...overrides,
	}
}

describe('SeatBox general seat font fitting', () => {
	const originalGetContext = HTMLCanvasElement.prototype.getContext
	const originalDocumentFonts = document.fonts

	function mockMeasureTextWithWidths(widths: number[]) {
		const measureText = jest.fn(() => {
			const width = widths.shift()
			if (width === undefined) {
				throw new Error('measureText width queue is empty')
			}
			return { width } as TextMetrics
		})

		Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
			configurable: true,
			value: jest.fn(
				() =>
					({
						font: '',
						measureText,
					}) as unknown as CanvasRenderingContext2D,
			),
		})

		return measureText
	}

	afterEach(() => {
		Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
			configurable: true,
			value: originalGetContext,
		})
		Object.defineProperty(document, 'fonts', {
			configurable: true,
			value: originalDocumentFonts,
		})
		jest.restoreAllMocks()
	})

	test('remeasures general work name after web fonts are ready', async () => {
		const fontsReady = createDeferredPromise<void>()
		Object.defineProperty(document, 'fonts', {
			configurable: true,
			value: {
				ready: fontsReady.promise,
			},
		})
		mockMeasureTextWithWidths([220, 160])
		const SeatBox = loadSeatBox()

		render(
			<SeatBox
				{...createBaseProps({
					processingSeat: {
						...createBaseProps().processingSeat,
						work_name: 'おかえりなさいませ👋',
					},
				})}
			/>,
		)

		const workName = await screen.findByText('おかえりなさいませ👋')
		const expectedFallbackFontSizePx = expectedFontSizePx({
			text: 'おかえりなさいませ👋',
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			lineWidthPx: GENERAL_SEAT_LINE_WIDTH_PX,
			measuredWidthPx: 220,
			baseEm: 0.95,
			minEm: 0.63,
		})
		const expectedLoadedFontSizePx = expectedFontSizePx({
			text: 'おかえりなさいませ👋',
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			lineWidthPx: GENERAL_SEAT_LINE_WIDTH_PX,
			measuredWidthPx: 160,
			baseEm: 0.95,
			minEm: 0.63,
		})

		await waitFor(() => {
			expect(fontSizePxOf(workName)).toBeCloseTo(expectedFallbackFontSizePx, 5)
		})

		await act(async () => {
			fontsReady.resolve()
			await fontsReady.promise
		})

		await waitFor(() => {
			expect(fontSizePxOf(workName)).toBeCloseTo(expectedLoadedFontSizePx, 5)
		})
	})

	test('remeasures display name when work name is empty', async () => {
		const fontsReady = createDeferredPromise<void>()
		Object.defineProperty(document, 'fonts', {
			configurable: true,
			value: {
				ready: fontsReady.promise,
			},
		})
		mockMeasureTextWithWidths([240, 150])
		const SeatBox = loadSeatBox()

		render(
			<SeatBox
				{...createBaseProps({
					processingSeat: {
						...createBaseProps().processingSeat,
						work_name: '',
						user_display_name: 'おかえりなさいませ👋',
					},
				})}
			/>,
		)

		const displayName = await screen.findByText('おかえりなさいませ👋')
		const expectedFallbackFontSizePx = expectedFontSizePx({
			text: 'おかえりなさいませ👋',
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			lineWidthPx: GENERAL_SEAT_LINE_WIDTH_PX,
			measuredWidthPx: 240,
			baseEm: 0.8,
			minEm: 0.5,
		})
		const expectedLoadedFontSizePx = expectedFontSizePx({
			text: 'おかえりなさいませ👋',
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			lineWidthPx: GENERAL_SEAT_LINE_WIDTH_PX,
			measuredWidthPx: 150,
			baseEm: 0.8,
			minEm: 0.5,
		})

		await waitFor(() => {
			expect(fontSizePxOf(displayName)).toBeCloseTo(
				expectedFallbackFontSizePx,
				5,
			)
		})

		await act(async () => {
			fontsReady.resolve()
			await fontsReady.promise
		})

		await waitFor(() => {
			expect(fontSizePxOf(displayName)).toBeCloseTo(expectedLoadedFontSizePx, 5)
		})
	})

	test('keeps rendering safely when document.fonts is unavailable', async () => {
		Object.defineProperty(document, 'fonts', {
			configurable: true,
			value: undefined,
		})
		mockMeasureTextWithWidths([220])
		const SeatBox = loadSeatBox()

		render(
			<SeatBox
				{...createBaseProps({
					processingSeat: {
						...createBaseProps().processingSeat,
						work_name: 'おかえりなさいませ👋',
					},
				})}
			/>,
		)

		const workName = await screen.findByText('おかえりなさいませ👋')
		const expectedFontSize = expectedFontSizePx({
			text: 'おかえりなさいませ👋',
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			lineWidthPx: GENERAL_SEAT_LINE_WIDTH_PX,
			measuredWidthPx: 220,
			baseEm: 0.95,
			minEm: 0.63,
		})

		await waitFor(() => {
			expect(fontSizePxOf(workName)).toBeCloseTo(expectedFontSize, 5)
		})
	})
})
