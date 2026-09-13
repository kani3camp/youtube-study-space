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

function firestoreSeatWithAppearance(appearance: Record<string, unknown>) {
	return {
		'seat-id': 1,
		'user-id': 'user-1',
		'user-display-name': 'ユーザー',
		'work-name': '作業',
		'break-work-name': '',
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

const legacyAppearance = {
	'color-code1': '#111111',
	'color-code2': '#222222',
	'num-stars': 3,
	'color-gradient-enabled': true,
}

describe('firestoreSeatConverter appearance migration contract', () => {
	test('reads a V1 appearance without inferring V2 from field values', () => {
		const seat = firestoreSeatConverter.fromFirestore(
			snapshotWith(
				firestoreSeatWithAppearance({
					...legacyAppearance,
					'top-bar-color': '#ABCDEF',
					rank: 5,
					'rank-visible': true,
				}),
			),
			{} as SnapshotOptions,
		)

		expect(seat.appearance).toEqual({
			schema_version: undefined,
			color_code1: '#111111',
			color_code2: '#222222',
			num_stars: 3,
			color_gradient_enabled: true,
		})
	})

	test('reads V2 canonical fields only when schema-version is exactly 2', () => {
		const seat = firestoreSeatConverter.fromFirestore(
			snapshotWith(
				firestoreSeatWithAppearance({
					...legacyAppearance,
					'schema-version': 2,
					'top-bar-color': '#ABCDEF',
					rank: 5,
					'rank-visible': true,
				}),
			),
			{} as SnapshotOptions,
		)

		expect(seat.appearance).toEqual({
			schema_version: 2,
			top_bar_color: '#ABCDEF',
			rank: 5,
			rank_visible: true,
			color_code1: '#111111',
			color_code2: '#222222',
			num_stars: 3,
			color_gradient_enabled: true,
		})
	})

	test('reports an unsupported schema-version and uses the legacy fallback', () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
		const seat = firestoreSeatConverter.fromFirestore(
			snapshotWith(
				firestoreSeatWithAppearance({
					...legacyAppearance,
					'schema-version': 3,
					'top-bar-color': '#ABCDEF',
					rank: 9,
					'rank-visible': true,
				}),
			),
			{} as SnapshotOptions,
		)

		expect(seat.appearance.schema_version).toBe(3)
		expect(seat.appearance.top_bar_color).toBeUndefined()
		expect(seat.appearance.rank).toBeUndefined()
		expect(seat.appearance.rank_visible).toBeUndefined()
		expect(warn).toHaveBeenCalledWith(
			'Unsupported SeatAppearance schema-version 3 for seat 1; using legacy appearance fallback',
		)
		warn.mockRestore()
	})

	test('writes symmetric V1 legacy field names', () => {
		const seat = baseSeat({
			color_code1: '#111111',
			color_code2: '#222222',
			num_stars: 3,
			color_gradient_enabled: true,
		})

		const written = firestoreSeatConverter.toFirestore(seat)

		expect(written.appearance).toEqual(legacyAppearance)
	})

	test('writes V2 canonical and legacy compatibility fields together', () => {
		const seat = baseSeat({
			schema_version: 2,
			top_bar_color: '#ABCDEF',
			rank: 5,
			rank_visible: true,
			color_code1: '#111111',
			color_code2: '#222222',
			num_stars: 3,
			color_gradient_enabled: true,
		})

		const written = firestoreSeatConverter.toFirestore(seat)

		expect(written.appearance).toEqual({
			...legacyAppearance,
			'schema-version': 2,
			'top-bar-color': '#ABCDEF',
			rank: 5,
			'rank-visible': true,
		})
	})
})

function baseSeat(appearance: Seat['appearance']): Seat {
	return {
		seat_id: 1,
		user_id: 'user-1',
		user_display_name: 'ユーザー',
		work_name: '作業',
		break_work_name: '',
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
