import { render, screen } from '@testing-library/react'
import { Timestamp } from 'firebase/firestore'
import { act } from 'react'
import type { Seat } from '../types/api'
import type { RoomLayout } from '../types/room-layout'
import type { SeatProps } from './SeatBox'
import SeatsPage from './SeatsPage'

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

jest.mock('./SeatBox', () => ({
	__esModule: true,
	default: ({ globalSeatId, minutesElapsed, minutesRemaining }: SeatProps) => (
		<div data-testid={`seat-${globalSeatId}`}>
			<span data-testid="minutes-elapsed">{minutesElapsed}</span>
			<span data-testid="minutes-remaining">{minutesRemaining}</span>
		</div>
	),
}))

const roomLayout: RoomLayout = {
	floor_image: '',
	font_size_ratio: 0.02,
	room_shape: { width: 100, height: 100 },
	seat_shape: { width: 40, height: 30 },
	partition_shapes: [],
	seats: [{ id: 1, x: 0, y: 0, rotate: 0 }],
	partitions: [],
}

const createSeat = (now: number): Seat => ({
	seat_id: 1,
	user_id: 'user-1',
	user_display_name: 'User',
	work_name: 'Work',
	break_work_name: '',
	entered_at: Timestamp.fromMillis(now - 3 * 60 * 1000),
	until: Timestamp.fromMillis(now + 57 * 60 * 1000),
	appearance: {
		color_code1: '#000000',
		color_code2: '#ffffff',
		num_stars: 0,
		color_gradient_enabled: false,
	},
	menu_code: '',
	state: 'work',
	current_state_started_at: Timestamp.fromMillis(now - 3 * 60 * 1000),
	current_state_until: Timestamp.fromMillis(now + 57 * 60 * 1000),
	cumulative_work_sec: 0,
	daily_cumulative_work_sec: 0,
	user_profile_image_url: '',
})

describe('SeatsPage', () => {
	beforeEach(() => {
		jest.useFakeTimers()
	})

	afterEach(() => {
		jest.useRealTimers()
	})

	test('updates elapsed and remaining time as minutes pass', () => {
		const initialNow = new Date('2026-01-01T00:00:00.000Z').valueOf()
		jest.setSystemTime(initialNow)

		render(
			<SeatsPage
				roomLayout={roomLayout}
				usedSeats={[createSeat(initialNow)]}
				firstSeatId={1}
				display={true}
				memberOnly={true}
				menuImageMap={new Map()}
				viewport={{ width: 100, height: 100 }}
			/>,
		)

		expect(screen.getByTestId('minutes-elapsed')).toHaveTextContent('3')
		expect(screen.getByTestId('minutes-remaining')).toHaveTextContent('57')

		act(() => {
			jest.advanceTimersByTime(60 * 1000)
		})

		expect(screen.getByTestId('minutes-elapsed')).toHaveTextContent('4')
		expect(screen.getByTestId('minutes-remaining')).toHaveTextContent('56')
	})
})
