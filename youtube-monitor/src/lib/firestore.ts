import { FirebaseOptions } from 'firebase/app'
import {
    DocumentData,
    FirestoreDataConverter,
    QueryDocumentSnapshot,
    SnapshotOptions,
} from 'firebase/firestore'
import { Seat, Menu } from '../types/api'
import { validateString } from './common'

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

export type SystemConstants = {
    max_seats: number
    member_max_seats: number
    min_vacancy_rate: number
    youtube_membership_enabled: boolean
    fixed_max_seats_enabled: boolean
}

export const firestoreConstantsConverter: FirestoreDataConverter<SystemConstants> = {
    toFirestore(constants: SystemConstants): DocumentData {
        return {
            'max-seats': constants.max_seats,
            'member-max-seats': constants.member_max_seats,
            'min-vacancy-rate': constants.min_vacancy_rate,
            'youtube-membership-enabled': constants.youtube_membership_enabled,
            'fixed-max-seats-enabled': constants.fixed_max_seats_enabled,
        }
    },
    fromFirestore(snapshot: QueryDocumentSnapshot, options: SnapshotOptions): SystemConstants {
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
        return {
            'seat-id': seat.seat_id,
            'user-id': seat.user_id,
            'user-display-name': seat.user_display_name,
            'work-name': seat.work_name,
            'break-work-name': seat.break_work_name,
            'entered-at': seat.entered_at,
            until: seat.until,
            appearance: {
                'color-code': seat.appearance.color_code1,
                'num-stars': seat.appearance.num_stars,
                'glow-animation': seat.appearance.color_gradient_enabled,
            },
            'menu-code': seat.menu_code,
            state: seat.state,
            'current-state-started-at': seat.current_state_started_at,
            'current-state-until': seat.current_state_until,
            'cumulative-work-sec': seat.cumulative_work_sec,
            'daily-cumulative-work-sec': seat.daily_cumulative_work_sec,
        }
    },
    fromFirestore(snapshot: QueryDocumentSnapshot, options: SnapshotOptions): Seat {
        const data = snapshot.data(options)
        return {
            seat_id: data['seat-id'],
            user_id: data['user-id'],
            user_display_name: data['user-display-name'],
            work_name: data['work-name'],
            break_work_name: data['break-work-name'],
            entered_at: data['entered-at'],
            until: data.until,
            appearance: {
                color_code1: data.appearance['color-code1'],
                color_code2: data.appearance['color-code2'],
                num_stars: data.appearance['num-stars'],
                color_gradient_enabled: data.appearance['color-gradient-enabled'],
            },
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
        }
    },
    fromFirestore(snapshot: QueryDocumentSnapshot, options: SnapshotOptions): Menu {
        const data = snapshot.data(options)
        return {
            code: data.code,
            name: data.name,
        }
    },
}
