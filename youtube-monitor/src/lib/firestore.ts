import {
	type FirebaseApp,
	type FirebaseOptions,
	getApp,
	getApps,
	initializeApp,
} from 'firebase/app'
import type {
	DocumentData,
	FirestoreDataConverter,
	QueryDocumentSnapshot,
	SnapshotOptions,
} from 'firebase/firestore'
import type { Menu, Seat, WorkNameTrend } from '../types/api'
import { validateString } from './common'
import {
	classifySeatAppearanceSchemaVersion,
	seatAppearanceV2SchemaVersion,
} from './seat-appearance-schema'

export const getFirebaseConfig = (): FirebaseOptions => {
	if (!validateString(process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID)) {
		alert('NEXT_PUBLIC_FIREBASE_PROJECT_ID is not valid.')
	}
	if (!validateString(process.env.NEXT_PUBLIC_FIREBASE_API_KEY)) {
		alert('NEXT_PUBLIC_FIREBASE_API_KEY is not valid.')
	}
	return {
		apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
		projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
	}
}

export const getFirebaseApp = (): FirebaseApp => {
	return getApps().length === 0 ? initializeApp(getFirebaseConfig()) : getApp()
}

export type SystemConstants = {
	max_seats: number
	member_max_seats: number
	min_vacancy_rate: number
	youtube_membership_enabled: boolean
	fixed_max_seats_enabled: boolean
}

export const firestoreConstantsConverter: FirestoreDataConverter<SystemConstants> =
	{
		toFirestore(constants: SystemConstants): DocumentData {
			return {
				'max-seats': constants.max_seats,
				'member-max-seats': constants.member_max_seats,
				'min-vacancy-rate': constants.min_vacancy_rate,
				'youtube-membership-enabled': constants.youtube_membership_enabled,
				'fixed-max-seats-enabled': constants.fixed_max_seats_enabled,
			}
		},
		fromFirestore(
			snapshot: QueryDocumentSnapshot,
			options: SnapshotOptions,
		): SystemConstants {
			const data = snapshot.data(options)
			return {
				max_seats: data['max-seats'],
				member_max_seats: data['member-max-seats'],
				min_vacancy_rate: data['min-vacancy-rate'],
				youtube_membership_enabled: data['youtube-membership-enabled'],
				fixed_max_seats_enabled: data['fixed-max-seats-enabled'],
			}
		},
	}

export const firestoreSeatConverter: FirestoreDataConverter<Seat> = {
	toFirestore(seat: Seat): DocumentData {
		const appearance = seat.appearance
		const firestoreAppearance: DocumentData = {
			'color-code1': appearance.color_code1,
			'color-code2': appearance.color_code2,
			'num-stars': appearance.num_stars,
			'color-gradient-enabled': appearance.color_gradient_enabled,
		}
		if (appearance.schema_version !== undefined) {
			firestoreAppearance['schema-version'] = appearance.schema_version
		}
		if (appearance.schema_version === seatAppearanceV2SchemaVersion) {
			firestoreAppearance['top-bar-color'] = appearance.top_bar_color
			firestoreAppearance.rank = appearance.rank
			firestoreAppearance['rank-visible'] = appearance.rank_visible
		}

		return {
			'seat-id': seat.seat_id,
			'user-id': seat.user_id,
			'user-display-name': seat.user_display_name,
			'work-name': seat.work_name,
			'break-work-name': seat.break_work_name,
			'entered-at': seat.entered_at,
			until: seat.until,
			appearance: firestoreAppearance,
			'menu-code': seat.menu_code,
			state: seat.state,
			'current-state-started-at': seat.current_state_started_at,
			'current-state-until': seat.current_state_until,
			'cumulative-work-sec': seat.cumulative_work_sec,
			'daily-cumulative-work-sec': seat.daily_cumulative_work_sec,
		}
	},
	fromFirestore(
		snapshot: QueryDocumentSnapshot,
		options: SnapshotOptions,
	): Seat {
		const data = snapshot.data(options)
		const appearance = data.appearance ?? {}
		const schemaVersion = appearance['schema-version']
		const schemaKind = classifySeatAppearanceSchemaVersion(schemaVersion)
		if (schemaKind === 'unsupported') {
			console.warn(
				`Unsupported SeatAppearance schema-version ${String(schemaVersion)} for seat ${String(data['seat-id'])}; using legacy appearance fallback`,
			)
		}
		const convertedAppearance = {
			schema_version:
				typeof schemaVersion === 'number' ? schemaVersion : undefined,
			color_code1: appearance['color-code1'],
			color_code2: appearance['color-code2'],
			num_stars: appearance['num-stars'],
			color_gradient_enabled: appearance['color-gradient-enabled'],
		}
		if (schemaKind === 'v2') {
			Object.assign(convertedAppearance, {
				top_bar_color: appearance['top-bar-color'],
				rank: appearance.rank,
				rank_visible: appearance['rank-visible'],
			})
		}

		return {
			seat_id: data['seat-id'],
			user_id: data['user-id'],
			user_display_name: data['user-display-name'],
			work_name: data['work-name'],
			break_work_name: data['break-work-name'],
			entered_at: data['entered-at'],
			until: data.until,
			appearance: convertedAppearance,
			menu_code: data['menu-code'],
			state: data.state,
			current_state_started_at: data['current-state-started-at'],
			current_state_until: data['current-state-until'],
			cumulative_work_sec: data['cumulative-work-sec'],
			daily_cumulative_work_sec: data['daily-cumulative-work-sec'],
			user_profile_image_url: data['user-profile-image-url'],
		}
	},
}

export const firestoreMenuConverter: FirestoreDataConverter<Menu> = {
	toFirestore(menu: Menu): DocumentData {
		return {
			code: menu.code,
			name: menu.name,
			image: menu.image,
		}
	},
	fromFirestore(
		snapshot: QueryDocumentSnapshot,
		options: SnapshotOptions,
	): Menu {
		const data = snapshot.data(options)
		return {
			code: data.code,
			name: data.name,
			image: data.image ?? '',
		}
	},
}

export const firestoreWorkNameTrendConverter: FirestoreDataConverter<WorkNameTrend> =
	{
		toFirestore(workNameTrend: WorkNameTrend): DocumentData {
			return {
				ranking: workNameTrend.ranking,
				'ranked-at': workNameTrend.ranked_at,
			}
		},
		fromFirestore(
			snapshot: QueryDocumentSnapshot,
			options: SnapshotOptions,
		): WorkNameTrend {
			const data = snapshot.data(options)
			return {
				ranking: data.ranking,
				ranked_at: data['ranked-at'],
			}
		},
	}
