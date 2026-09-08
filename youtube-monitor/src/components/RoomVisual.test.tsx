import { render, screen } from '@testing-library/react'
import type { ComponentPropsWithoutRef } from 'react'
import RoomVisual from './RoomVisual'

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
				floorImage="/images/rooms/test-room.png"
				width={1520}
				height={900}
			/>,
		)

		const image = screen.getByRole('img', { name: 'room image' })
		expect(image).toHaveAttribute(
		'data-src',
		'/images/rooms/test-room.png',
	)
		expect(image).toHaveAttribute('data-width', '1520')
		expect(image).toHaveAttribute('data-height', '900')
		expect(image).not.toHaveAttribute('data-filter')
	})

	test('keeps rendering the static floor image when a scene config is present', () => {
		render(
			<RoomVisual
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

	test('renders nothing when the room has no floor image', () => {
		const { container } = render(
			<RoomVisual floorImage="" width={1520} height={900} />,
		)

		expect(container).toBeEmptyDOMElement()
	})
})
