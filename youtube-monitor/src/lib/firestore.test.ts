import type {
	QueryDocumentSnapshot,
	SnapshotOptions,
	Timestamp,
} from 'firebase/firestore'
import type { Seat } from '../types/api'
import { firestoreSeatConverter } from './firestore'

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

const timestamp = {} as Timestamp

function snapshotWith(data: Record<string, unknown>): QueryDocumentSnapshot {
	return {
		data: vi.fn(() => data),
	} as unknown as QueryDocumentSnapshot
}

function firestoreSeatWithAppearance(appearance: unknown) {
	return {
		'seat-id': 1,
		'user-id': 'user-1',
		'user-display-name': 'ユーザー',
		'work-name': '作業',
		'entered-at': timestamp,
		until: timestamp,
		appearance,
		'menu-code': '',
		state: 'work',
		'current-state-started-at': timestamp,
		'current-state-until': timestamp,
		'cumulative-work-sec': 0,
		'daily-cumulative-work-sec': 0,
		'user-profile-image-url': '',
	}
}

const canonicalAppearance = {
	'schema-version': 2,
	'top-bar-color': '#ABCDEF',
	rank: 5,
	'rank-visible': true,
	'num-stars': 3,
}

describe('firestoreSeatConverter SeatAppearance V2 contract', () => {
	test('reads V2 canonical fields', () => {
		const seat = firestoreSeatConverter.fromFirestore(
			snapshotWith(firestoreSeatWithAppearance(canonicalAppearance)),
			{} as SnapshotOptions,
		)

		expect(seat.appearance).toEqual({
			schema_version: 2,
			top_bar_color: '#ABCDEF',
			rank: 5,
			rank_visible: true,
			num_stars: 3,
		})
	})

	test('ignores Release 1 legacy fields on a V2 document', () => {
		const seat = firestoreSeatConverter.fromFirestore(
			snapshotWith(
				firestoreSeatWithAppearance({
					...canonicalAppearance,
					'color-code1': '#111111',
					'color-code2': '#222222',
					'color-gradient-enabled': true,
				}),
			),
			{} as SnapshotOptions,
		)

		expect(seat.appearance).toEqual({
			schema_version: 2,
			top_bar_color: '#ABCDEF',
			rank: 5,
			rank_visible: true,
			num_stars: 3,
		})
	})

	test.each([undefined, null])(
		'rejects a missing appearance instead of using a V1 fallback (%s)',
		(appearance) => {
			expect(() =>
				firestoreSeatConverter.fromFirestore(
					snapshotWith(firestoreSeatWithAppearance(appearance)),
					{} as SnapshotOptions,
				),
			).toThrow('expected schema-version 2')
		},
	)

	test('rejects an unsupported schema-version instead of falling back', () => {
		expect(() =>
			firestoreSeatConverter.fromFirestore(
				snapshotWith(
					firestoreSeatWithAppearance({
						...canonicalAppearance,
						'schema-version': 3,
						'color-code1': '#111111',
					}),
				),
				{} as SnapshotOptions,
			),
		).toThrow('Unsupported SeatAppearance schema-version 3')
	})

	test.each([
		{ name: 'top-bar-color', patch: { 'top-bar-color': undefined } },
		{ name: 'rank', patch: { rank: undefined } },
		{ name: 'rank-visible', patch: { 'rank-visible': undefined } },
		{ name: 'num-stars', patch: { 'num-stars': undefined } },
	])('rejects a malformed V2 appearance missing $name', ({ patch }) => {
		expect(() =>
			firestoreSeatConverter.fromFirestore(
				snapshotWith(
					firestoreSeatWithAppearance({
						...canonicalAppearance,
						...patch,
					}),
				),
				{} as SnapshotOptions,
			),
		).toThrow('Malformed SeatAppearance V2 for seat 1')
	})

	test('writes only V2 canonical fields', () => {
		const written = firestoreSeatConverter.toFirestore(
			baseSeat({
				schema_version: 2,
				top_bar_color: '#ABCDEF',
				rank: 5,
				rank_visible: true,
				num_stars: 3,
			}),
		)

		expect(written.appearance).toEqual(canonicalAppearance)
		expect(written.appearance).not.toHaveProperty('color-code1')
		expect(written.appearance).not.toHaveProperty('color-code2')
		expect(written.appearance).not.toHaveProperty('color-gradient-enabled')
		expect(written).not.toHaveProperty('break-work-name')
	})
})

function baseSeat(appearance: Seat['appearance']): Seat {
	return {
		seat_id: 1,
		user_id: 'user-1',
		user_display_name: 'ユーザー',
		work_name: '作業',
		entered_at: timestamp,
		until: timestamp,
		appearance,
		menu_code: '',
		state: 'work',
		current_state_started_at: timestamp,
		current_state_until: timestamp,
		cumulative_work_sec: 0,
		daily_cumulative_work_sec: 0,
		user_profile_image_url: '',
	}
}
