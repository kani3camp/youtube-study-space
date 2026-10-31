import { act, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { Monitor } from '../pages/index'

const firestoreMocks = vi.hoisted(() => ({
	subscriptions: new Map<
		string,
		{ fail: () => void; unsubscribe: ReturnType<typeof vi.fn> }
	>(),
}))

vi.mock('firebase/firestore', async (importOriginal) => ({
	...(await importOriginal<typeof import('firebase/firestore')>()),
	collection: vi.fn((_db, path: string) => ({ path })),
	doc: vi.fn((_db, collectionName: string, id: string) => ({
		withConverter: () => ({ path: `${collectionName}/${id}` }),
	})),
	getFirestore: vi.fn(() => ({})),
	onSnapshot: vi.fn(({ path }: { path: string }, _next, fail: () => void) => {
		const unsubscribe = vi.fn()
		firestoreMocks.subscriptions.set(path, { fail, unsubscribe })
		return unsubscribe
	}),
	orderBy: vi.fn(() => ({})),
	query: vi.fn((reference) => ({ withConverter: () => reference })),
}))

vi.mock('next/router', () => ({
	useRouter: () => ({ query: {} }),
}))
vi.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: () => ({ style: { fontFamily: 'mock-font' } }),
	Source_Code_Pro: () => ({ style: { fontFamily: 'mock-code-font' } }),
}))

vi.mock('./AmbientFrame', () => ({ default: () => null }))
vi.mock('./BackgroundImage', () => ({ default: () => null }))
vi.mock('./BgmPlayer', () => ({ default: () => null }))
vi.mock('./Clock', () => ({ default: () => null }))
vi.mock('./ColorBar', () => ({ default: () => null }))
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
	firestoreConstantsConverter: {},
	firestoreMenuConverter: {},
	firestoreSeatConverter: {},
	firestoreWorkNameTrendConverter: {},
	getFirebaseApp: vi.fn(() => ({})),
}))
vi.mock('../styles/AmbientFrame.styles', () => ({ themedRoot: {} }))

describe('Monitor Firestore failure boundary', () => {
	beforeEach(() => {
		firestoreMocks.subscriptions.clear()
		process.env.NEXT_PUBLIC_API_KEY = 'ci-dummy-api-key'
	})

	const readPaths = [
		'menu',
		'seats',
		'member-seats',
		'work-name-trend',
		'config/constants',
	]

	test.each(readPaths)(
		'stops all Firestore listeners when %s fails',
		(path) => {
			render(<Monitor />)
			expect([...firestoreMocks.subscriptions.keys()].sort()).toEqual(
				[...readPaths].sort(),
			)

			act(() => {
				firestoreMocks.subscriptions.get(path)?.fail()
			})

			expect(screen.getByText('Firestore を読み込めません')).toBeInTheDocument()
			for (const { unsubscribe } of firestoreMocks.subscriptions.values()) {
				expect(unsubscribe).toHaveBeenCalledOnce()
			}
		},
	)

	test('stops all Firestore listeners when the monitor unmounts', () => {
		const view = render(<Monitor />)
		view.unmount()

		expect(firestoreMocks.subscriptions.size).toBe(readPaths.length)
		for (const { unsubscribe } of firestoreMocks.subscriptions.values()) {
			expect(unsubscribe).toHaveBeenCalledOnce()
		}
	})
})
