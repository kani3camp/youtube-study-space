import { render, screen } from '@testing-library/react'
import type { ComponentPropsWithoutRef } from 'react'
import { useLiveRoomScenesEnabled } from '../hooks/use-live-room-scenes-enabled'
import RoomVisual from './RoomVisual'

jest.mock('../hooks/use-live-room-scenes-enabled', () => ({
	useLiveRoomScenesEnabled: jest.fn(() => true),
}))

const liveRoomScenesEnabledMock =
	useLiveRoomScenesEnabled as jest.MockedFunction<
		typeof useLiveRoomScenesEnabled
	>

jest.mock('./LivingSceneLayer', () => ({
	__esModule: true,
	default: ({ active, profile }: { active: boolean; profile: string }) => (
		<span
			data-testid="living-scene-layer"
			data-active={String(active)}
			data-profile={profile}
		/>
	),
}))

jest.mock('next/image', () => ({
	__esModule: true,
	default: (props: ComponentPropsWithoutRef<'img'>) => {
		const { alt, height, src, style, width } = props
		return (
			<span
				role="img"
				aria-label={alt}
				data-filter={style?.filter}
				data-height={height}
				data-src={typeof src === 'string' ? src : ''}
				data-width={width}
			/>
		)
	},
}))

describe('RoomVisual static compatibility', () => {
	beforeEach(() => {
		liveRoomScenesEnabledMock.mockReturnValue(true)
	})

	test('renders the static floor image when no scene is configured', () => {
		render(
			<RoomVisual
				active={true}
				floorImage="/images/rooms/test-room.png"
				width={1520}
				height={900}
			/>,
		)

		const image = screen.getByRole('img', { name: 'room image' })
		expect(image).toHaveAttribute('data-src', '/images/rooms/test-room.png')
		expect(image).toHaveAttribute('data-width', '1520')
		expect(image).toHaveAttribute('data-height', '900')
		expect(image).not.toHaveAttribute('data-filter')
	})

	test('color-grades an Ambient room while keeping its static floor image', () => {
		render(
			<RoomVisual
				active={true}
				floorImage="/images/rooms/test-room.png"
				scene={{ mode: 'ambient' }}
				width={1520}
				height={900}
			/>,
		)

		const image = screen.getByRole('img', { name: 'room image' })
		expect(image).toHaveAttribute('data-src', '/images/rooms/test-room.png')
		expect(image).toHaveAttribute(
			'data-filter',
			'var(--scene-ambient-filter, none)',
		)
	})

	test('color-grades the static fallback while a Living scene layer is active', () => {
		render(
			<RoomVisual
				active={true}
				floorImage="/images/rooms/test-room.png"
				scene={{ mode: 'living', profile: 'lume-rainy-poc' }}
				width={1520}
				height={900}
			/>,
		)

		expect(screen.getByRole('img', { name: 'room image' })).toHaveAttribute(
			'data-filter',
			'var(--scene-ambient-filter, none)',
		)
		const livingLayer = screen.getByTestId('living-scene-layer')
		expect(livingLayer).toHaveAttribute('data-active', 'true')
		expect(livingLayer).toHaveAttribute('data-profile', 'lume-rainy-poc')
	})

	test('renders only the ungraded static fallback while Live Room Scenes is disabled', () => {
		liveRoomScenesEnabledMock.mockReturnValue(false)
		render(
			<RoomVisual
				active={true}
				floorImage="/images/rooms/test-room.png"
				scene={{ mode: 'living', profile: 'lume-rainy-poc' }}
				width={1520}
				height={900}
			/>,
		)

		expect(screen.getByRole('img', { name: 'room image' })).not.toHaveAttribute(
			'data-filter',
		)
		expect(screen.queryByTestId('living-scene-layer')).not.toBeInTheDocument()
	})

	test('does not apply Ambient grading while Live Room Scenes is disabled', () => {
		liveRoomScenesEnabledMock.mockReturnValue(false)
		render(
			<RoomVisual
				active={true}
				floorImage="/images/rooms/test-room.png"
				scene={{ mode: 'ambient' }}
				width={1520}
				height={900}
			/>,
		)

		expect(screen.getByRole('img', { name: 'room image' })).not.toHaveAttribute(
			'data-filter',
		)
	})

	test('renders nothing when the room has no floor image', () => {
		const { container } = render(
			<RoomVisual active={true} floorImage="" width={1520} height={900} />,
		)

		expect(container).toBeEmptyDOMElement()
	})
})
