import { render, screen } from '@testing-library/react'
import useMonitorData from '../../hooks/useMonitorData'
import useRoomPages from '../../hooks/useRoomPages'
import HorizontalMonitor from './HorizontalMonitor'
import type { RoomPage } from './types'
import VerticalMonitor from './VerticalMonitor'

jest.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: jest.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: jest.fn(() => ({
		style: { fontFamily: 'Source Code Pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

jest.mock('next/router', () => ({
	useRouter: () => ({ query: { page: '1' } }),
}))

jest.mock('../../lib/constants', () => ({
	Constants: {
		screenWidth: 1920,
		screenHeight: 1080,
		sideBarWidth: 400,
		tickerWidth: 600,
		messageBarHeight: 80,
		pagingIntervalSeconds: 8,
	},
}))

jest.mock('../../hooks/use-time-theme', () => ({
	useTimeTheme: () => ({
		timeTheme: 'day',
		textTone: 'dark',
		contrastBridge: false,
	}),
}))

jest.mock('../../hooks/useMonitorData', () => ({
	__esModule: true,
	default: jest.fn(),
}))
jest.mock('../../hooks/useRoomPages', () => ({
	__esModule: true,
	default: jest.fn(),
}))
jest.mock('../../hooks/useSeatCapacityController', () => jest.fn())
jest.mock('../../hooks/useSynchronizedPage', () => ({
	getFixedPageIndex: jest.fn(() => 0),
	useSynchronizedPage: jest.fn(() => 0),
}))

jest.mock('../AmbientFrame', () => () => null)
jest.mock('../BackgroundImage', () => () => null)
jest.mock('../CenterLoading', () => () => <div>loading</div>)
jest.mock('../Clock', () => ({ variant }: { variant?: string }) => (
	<div data-testid="clock" data-variant={variant ?? 'horizontal'} />
))
jest.mock('../ColorBar', () => () => null)
jest.mock('../MenuDisplay', () => () => null)
jest.mock('../Message', () => ({ variant }: { variant?: string }) => (
	<div data-testid="message" data-variant={variant ?? 'horizontal'} />
))
jest.mock('../Timer', () => ({ variant }: { variant?: string }) => (
	<div data-testid="timer" data-variant={variant ?? 'horizontal'} />
))
jest.mock('../Usage', () => ({ variant }: { variant?: string }) => (
	<div data-testid="usage" data-variant={variant ?? 'horizontal'} />
))
jest.mock('../BgmPlayer', () => () => <div data-testid="bgm" />)
jest.mock('../TickerBoard', () => () => <div data-testid="ticker" />)
jest.mock(
	'./RoomStage',
	() =>
		({ viewport }: { viewport: { width: number; height: number } }) => (
			<div
				data-testid="room-stage"
				data-width={viewport.width}
				data-height={viewport.height}
			/>
		),
)

const page: RoomPage = {
	roomLayout: {
		floor_image: '/room.png',
		font_size_ratio: 0.02,
		room_shape: { width: 1520, height: 1000 },
		seat_shape: { width: 200, height: 100 },
		partition_shapes: [],
		seats: [],
		partitions: [],
	},
	usedSeats: [],
	firstSeatId: 1,
	memberOnly: true,
}

const monitorData: ReturnType<typeof useMonitorData> = {
	generalSeats: [],
	memberSeats: [],
	systemConstants: undefined,
	seatCapacityControlReady: false,
	menuImageMap: new Map<string, string>(),
	menuItems: [],
	workNameTrend: {} as ReturnType<typeof useMonitorData>['workNameTrend'],
	loading: false,
}

beforeEach(() => {
	jest.mocked(useMonitorData).mockReturnValue(monitorData)
	jest.mocked(useRoomPages).mockReturnValue([page])
})

it('renders the vertical monitor as a non-overlapping flow without BGM or ticker', () => {
	const { container } = render(<VerticalMonitor />)
	const sections = Array.from(
		container.querySelectorAll('[data-vertical-section]'),
	).map((element) => element.getAttribute('data-vertical-section'))

	expect(sections).toEqual(['safe-area', 'room', 'status', 'timer', 'commands'])
	expect(screen.getByTestId('room-stage')).toHaveAttribute('data-width', '1080')
	expect(Number(screen.getByTestId('room-stage').dataset.height)).toBeCloseTo(
		(1000 / 1520) * 1080,
	)
	expect(screen.queryByTestId('bgm')).not.toBeInTheDocument()
	expect(screen.queryByTestId('ticker')).not.toBeInTheDocument()
})

it('keeps BGM and work trends in the horizontal monitor', () => {
	render(<HorizontalMonitor />)

	expect(screen.getByTestId('bgm')).toBeInTheDocument()
	expect(screen.getByTestId('ticker')).toBeInTheDocument()
})
