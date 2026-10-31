import { fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { Monitor } from '../pages/index'

const firestoreMocks = vi.hoisted(() => ({
	menuUnsubscribe: vi.fn(),
}))

vi.mock('firebase/firestore', () => ({
	collection: vi.fn(() => ({})),
	getFirestore: vi.fn(() => ({})),
	onSnapshot: vi.fn(() => firestoreMocks.menuUnsubscribe),
	orderBy: vi.fn(() => ({})),
	query: vi.fn(() => ({ withConverter: vi.fn(() => ({})) })),
}))

vi.mock('./AmbientFrame', () => ({ default: () => null }))
vi.mock('./BackgroundImage', () => ({ default: () => null }))
vi.mock('./BgmPlayer', () => ({ default: () => null }))
vi.mock('./Clock', () => ({ default: () => null }))
vi.mock('./ColorBar', () => ({ default: () => null }))
vi.mock('./MainContent', () => ({
	default: ({ onReadError }: { onReadError: () => void }) => (
		<button type="button" onClick={onReadError}>
			fail a seat listener
		</button>
	),
}))
vi.mock('./MenuDisplay', () => ({ default: () => null }))
vi.mock('./MonitorAuthGate', () => ({
	default: ({ children }: { children: ReactNode }) => children,
}))
vi.mock('./Timer', () => ({ default: () => null }))
vi.mock('./Usage', () => ({ default: () => null }))
vi.mock('../hooks/use-time-theme', () => ({
	useTimeTheme: () => ({
		timeTheme: 'day',
		textTone: 'dark',
		contrastBridge: false,
	}),
}))
vi.mock('../lib/firestore', () => ({
	firestoreMenuConverter: {},
	getFirebaseApp: vi.fn(() => ({})),
}))
vi.mock('../styles/AmbientFrame.styles', () => ({ themedRoot: {} }))

describe('Monitor Firestore failure boundary', () => {
	beforeEach(() => {
		firestoreMocks.menuUnsubscribe.mockClear()
	})

	test('stops the menu listener when another Firestore listener fails', () => {
		render(<Monitor />)

		fireEvent.click(
			screen.getByRole('button', { name: 'fail a seat listener' }),
		)

		expect(firestoreMocks.menuUnsubscribe).toHaveBeenCalledOnce()
		expect(screen.getByText('Firestore を読み込めません')).toBeInTheDocument()
	})
})
