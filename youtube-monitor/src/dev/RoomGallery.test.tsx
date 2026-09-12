import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentPropsWithoutRef } from 'react'
import { getRoomGalleryEntries } from '../rooms/room-gallery'
import { roomRegistry } from '../rooms/room-registry'
import RoomGallery from './RoomGallery'

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
		const { alt, src } = props
		return (
			<span
				role="img"
				aria-label={alt}
				data-src={typeof src === 'string' ? src : ''}
				data-next-image=""
			/>
		)
	},
}))

beforeAll(() => {
	jest.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
})

test('renders every registry room under the two gallery sections', () => {
	render(<RoomGallery />)

	expect(
		screen.getByRole('heading', { name: 'Currently Enabled' }),
	).toBeVisible()
	expect(screen.getByRole('heading', { name: 'Other Rooms' })).toBeVisible()
	expect(screen.getAllByRole('button', { name: /\(.+\)$/ })).toHaveLength(
		Object.keys(roomRegistry).length,
	)
	expect(
		screen.getByRole('button', { name: 'Chabio 2 (chabio2)' }),
	).toBeVisible()
	expect(
		screen.getByRole('button', { name: 'Template (template)' }),
	).toBeVisible()
})

test('shows every applicable category and room kind on each card', () => {
	const entries = getRoomGalleryEntries('PROD')
	render(<RoomGallery />)

	for (const entry of entries) {
		const card = screen.getByRole('button', {
			name: `${entry.displayName} (${entry.id})`,
		})
		expect(card).toHaveTextContent(
			[...entry.categories, ...entry.kinds].join(' / ') || 'Uncategorized',
		)
	}
})

test('switches config and updates enabled room badges', async () => {
	const user = userEvent.setup()
	render(<RoomGallery />)

	const chabioCard = screen.getByRole('button', { name: 'Chabio 2 (chabio2)' })
	expect(within(chabioCard).getByText('Enabled')).toBeVisible()

	await user.click(screen.getByRole('button', { name: 'DEV' }))
	const updatedChabioCard = screen.getByRole('button', {
		name: 'Chabio 2 (chabio2)',
	})
	expect(within(updatedChabioCard).getByText('Disabled')).toBeVisible()
	expect(
		screen.getByRole('button', { name: 'Moon Night 1 (moonNight1)' }),
	).toHaveAttribute('aria-pressed', 'true')
})

test('switches profile and seat state controls', async () => {
	const user = userEvent.setup()
	render(<RoomGallery />)

	const memberButton = screen.getByRole('button', { name: 'Member' })
	await user.click(memberButton)
	expect(memberButton).toHaveAttribute('aria-pressed', 'true')

	const emptyButton = screen.getByRole('button', { name: 'Empty' })
	await user.click(emptyButton)
	expect(emptyButton).toHaveAttribute('aria-pressed', 'true')
	expect(screen.getByText('State')).toBeVisible()
})
