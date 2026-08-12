import { doc, onSnapshot } from 'firebase/firestore'
import {
	firestoreMonitorPublicConfigConverter,
	type MonitorPublicConfig,
	subscribeToMonitorPublicConfig,
} from './firestore'

jest.mock('firebase/firestore', () => ({
	doc: jest.fn(),
	onSnapshot: jest.fn(),
}))

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

const mockedDoc = jest.mocked(doc)
const mockedOnSnapshot = jest.mocked(onSnapshot)

const firstConfig: MonitorPublicConfig = {
	max_seats: 100,
	member_max_seats: 20,
	min_vacancy_rate: 0.25,
	youtube_membership_enabled: true,
	fixed_max_seats_enabled: false,
}

beforeEach(() => {
	jest.clearAllMocks()
})

test('monitor config converter maps exactly the five public fields', () => {
	expect(
		firestoreMonitorPublicConfigConverter.toFirestore(firstConfig),
	).toEqual({
		'max-seats': 100,
		'member-max-seats': 20,
		'min-vacancy-rate': 0.25,
		'youtube-membership-enabled': true,
		'fixed-max-seats-enabled': false,
	})

	const fromFirestore = firestoreMonitorPublicConfigConverter.fromFirestore(
		{
			data: () => ({
				'max-seats': 90,
				'member-max-seats': 18,
				'min-vacancy-rate': 0.3,
				'youtube-membership-enabled': false,
				'fixed-max-seats-enabled': true,
			}),
		} as never,
		{},
	)
	expect(fromFirestore).toEqual({
		max_seats: 90,
		member_max_seats: 18,
		min_vacancy_rate: 0.3,
		youtube_membership_enabled: false,
		fixed_max_seats_enabled: true,
	})
})

test('shared monitor subscription uses public-config/monitor and forwards realtime updates', () => {
	const convertedReference = { path: 'public-config/monitor' }
	const withConverter = jest.fn(() => convertedReference)
	mockedDoc.mockReturnValue({ withConverter } as never)
	let snapshotHandler:
		| ((snapshot: { data: () => MonitorPublicConfig }) => void)
		| undefined
	const unsubscribe = jest.fn()
	mockedOnSnapshot.mockImplementation(((_reference, onNext) => {
		snapshotHandler = onNext as typeof snapshotHandler
		return unsubscribe
	}) as never)
	const onConfig = jest.fn()
	const db = {} as never

	const returnedUnsubscribe = subscribeToMonitorPublicConfig(db, onConfig)

	expect(mockedDoc).toHaveBeenCalledWith(db, 'public-config', 'monitor')
	expect(withConverter).toHaveBeenCalledWith(
		firestoreMonitorPublicConfigConverter,
	)
	expect(mockedOnSnapshot).toHaveBeenCalledWith(
		convertedReference,
		expect.any(Function),
		undefined,
	)
	expect(returnedUnsubscribe).toBe(unsubscribe)

	snapshotHandler?.({ data: () => firstConfig })
	const updatedConfig = { ...firstConfig, max_seats: 120, member_max_seats: 24 }
	snapshotHandler?.({ data: () => updatedConfig })

	expect(onConfig).toHaveBeenNthCalledWith(1, firstConfig)
	expect(onConfig).toHaveBeenNthCalledWith(2, updatedConfig)
})
