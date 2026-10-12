export type ReasonCode =
	| 'SOURCE_UNAVAILABLE'
	| 'HISTORY_INCOMPLETE'
	| 'HISTORY_LIMIT_EXCEEDED'
	| 'DATA_INCONSISTENT'
	| 'METADATA_REFRESH_FAILED'
	| 'METADATA_TOO_OLD'

export type Metric =
	| { availability: 'available'; workSec: number; reasonCode: null }
	| { availability: 'unavailable'; workSec: null; reasonCode: ReasonCode }

export type Section<T> =
	| { availability: 'available'; reasonCode: null; data: T }
	| { availability: 'partial'; reasonCode: ReasonCode; data: T }
	| { availability: 'unavailable'; reasonCode: ReasonCode; data: null }

export type Current = {
	state: 'work' | 'break' | 'not_seated' | 'unregistered'
	workName: string | null
	roomType: 'standard' | 'member' | null
	seatNumber: number | null
	stateStartedAt: string | null
	expectedEndAt: string | null
}

export type Account = {
	displayName: string
	handle: string | null
	avatarUrl: string | null
}

export type MyPage = {
	generatedAt: string
	timezone: 'Asia/Tokyo'
	partial: boolean
	current: Section<Current>
	summary: Section<{ today: Metric; week: Metric; lifetime: Metric }>
	recent7Days: {
		availability: 'available' | 'partial' | 'unavailable'
		reasonCode: ReasonCode | null
		data: (Metric & { date: string })[]
	}
	account: Section<Account>
}
