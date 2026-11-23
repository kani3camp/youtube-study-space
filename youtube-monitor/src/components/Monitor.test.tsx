import { act, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { Monitor } from '../pages/index'

const firestoreMocks = vi.hoisted(() => ({
	subscriptions: new Map<
		string,
		{
			next: (snapshot: unknown) => void
			fail: (error?: unknown) => void
			unsubscribe: ReturnType<typeof vi.fn>
		}
	>(),
}))

vi.mock('firebase/firestore', async (importOriginal) => ({
	...(await importOriginal<typeof import('firebase/firestore')>()),
	collection: vi.fn((_db, path: string) => ({ path })),
	doc: vi.fn((_db, collectionName: string, id: string) => ({
		withConverter: () => ({ path: `${collectionName}/${id}` }),
	})),
	getFirestore: vi.fn(() => ({})),
	onSnapshot: vi.fn(
		(
			{ path }: { path: string },
			next: (snapshot: unknown) => void,
			fail: (error?: unknown) => void,
		) => {
			const unsubscribe = vi.fn()
			firestoreMocks.subscriptions.set(path, { next, fail, unsubscribe })
			return unsubscribe
		},
	),
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
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
	})
	afterEach(() => vi.unstubAllGlobals())

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
				firestoreMocks.subscriptions
					.get(path)
					?.fail({ code: 'permission-denied' })
			})

			expect(screen.getByText('配信画面を表示できません')).toBeInTheDocument()
			expect(fetch).toHaveBeenCalledExactlyOnceWith(
				'/api/monitor-auth-required',
				{
					method: 'POST',
					headers: { 'X-Monitor-Auth-Required': 'firestore-denied' },
					credentials: 'omit',
				},
			)
			for (const { unsubscribe } of firestoreMocks.subscriptions.values()) {
				expect(unsubscribe).toHaveBeenCalledOnce()
			}
		},
	)

	test('reports connectivity failures without diagnosing them as missing permissions', () => {
		render(<Monitor />)
		act(() =>
			firestoreMocks.subscriptions.get('menu')?.fail({ code: 'unavailable' }),
		)
		expect(fetch).toHaveBeenCalledExactlyOnceWith(
			'/api/monitor-auth-required',
			{
				method: 'POST',
				headers: { 'X-Monitor-Auth-Required': 'firestore-error' },
				credentials: 'omit',
			},
		)
		for (const { unsubscribe } of firestoreMocks.subscriptions.values()) {
			expect(unsubscribe).toHaveBeenCalledOnce()
		}
	})

	test('stops all Firestore listeners when the monitor unmounts', () => {
		const view = render(<Monitor />)
		view.unmount()

		expect(firestoreMocks.subscriptions.size).toBe(readPaths.length)
		for (const { unsubscribe } of firestoreMocks.subscriptions.values()) {
			expect(unsubscribe).toHaveBeenCalledOnce()
		}
	})

	test('routes a seat document that cannot be converted to the read error path', () => {
		vi.spyOn(console, 'error').mockImplementation(() => undefined)
		render(<Monitor />)
		act(() =>
			firestoreMocks.subscriptions.get('seats')?.next({
				docs: [
					{
						data: () => {
							throw new Error('unsupported seat schema')
						},
					},
				],
			}),
		)
		expect(screen.getByText('配信画面を表示できません')).toBeInTheDocument()
		expect(fetch).toHaveBeenCalledExactlyOnceWith(
			'/api/monitor-auth-required',
			expect.objectContaining({
				headers: { 'X-Monitor-Auth-Required': 'firestore-error' },
			}),
		)
	})

	test('routes a room config failure while reviewing max seats to the read error path', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => undefined)
		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', 'INVALID')
		render(<Monitor />)
		await act(async () =>
			firestoreMocks.subscriptions.get('config/constants')?.next({
				exists: () => true,
				data: () => ({
					max_seats: 10,
					member_max_seats: 5,
					min_vacancy_rate: 0.2,
					youtube_membership_enabled: true,
					fixed_max_seats_enabled: false,
				}),
			}),
		)
		expect(screen.getByText('配信画面を表示できません')).toBeInTheDocument()
		vi.unstubAllEnvs()
	})
})
