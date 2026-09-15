import { render, screen } from '@testing-library/react'
import type { SeatProps } from './SeatBox'

vi.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: vi.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: vi.fn(() => ({
		style: { fontFamily: 'mock-source-code-pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

test('renders a vacant seat without processingSeat', async () => {
	const { default: SeatBox } = await import('./SeatBox')
	const props: SeatProps = {
		globalSeatId: 123,
		isUsed: false,
		memberOnly: false,
		hoursRemaining: 0,
		minutesRemaining: 0,
		hoursElapsed: 0,
		minutesElapsed: 0,
		seatFontSizePx: 22.8,
		processingSeat: undefined,
		seatPosition: { x: 0, y: 0, rotate: 0 },
		seatShape: { widthPx: 140, heightPx: 100 },
		roomShape: { widthPx: 1520, heightPx: 1000 },
		menuImageMap: new Map<string, string>(),
	}

	render(<SeatBox {...props} />)

	expect(screen.getByText('!123')).toBeInTheDocument()
	expect(screen.queryByTestId('seat-accent-bar')).not.toBeInTheDocument()
})
