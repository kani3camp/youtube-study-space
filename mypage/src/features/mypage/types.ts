export type MyPageResponse = MyPageOkResponse | MyPageNotRegisteredResponse

export type MyPageOkResponse = {
	status: 'ok'
	viewer: Viewer
	stats: {
		dailyWorkSec: number
		cumulativeWorkSec: number
	}
	currentSeat: CurrentSeat | null
}

export type MyPageNotRegisteredResponse = {
	status: 'not_registered'
	viewer: Viewer
}

export type Viewer = {
	youtubeChannelId: string
	displayName: string
	profileImageUrl: string
}

export type CurrentSeat = {
	seatId: number
	isMemberSeat: boolean
	state: 'work' | 'break'
	workName: string
	breakWorkName: string
	startedAt: string
	until: string
}
