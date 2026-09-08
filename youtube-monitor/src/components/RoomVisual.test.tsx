import { render, screen } from '@testing-library/react'
import type { ComponentPropsWithoutRef } from 'react'
import RoomVisual from './RoomVisual'

jest.mock('./LivingSceneLayer', () => ({
	__esModule: true,
	default: ({
		active,
		profile,
	}: {
		active: boolean
		profile: string
	}) => (
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

	test('keeps rendering the static floor image when a scene config is present', () => {
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

	test('keeps the static fallback while a living scene layer is active', () => {
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
			'data-src',
			'/images/rooms/test-room.png',
		)
		const livingLayer = screen.getByTestId('living-scene-layer')
		expect(livingLayer).toHaveAttribute('data-active', 'true')
		expect(livingLayer).toHaveAttribute('data-profile', 'lume-rainy-poc')
	})

	test('renders nothing when the room has no floor image', () => {
		const { container } = render(
			<RoomVisual active={true} floorImage="" width={1520} height={900} />,
		)

		expect(container).toBeEmptyDOMElement()
	})
})
