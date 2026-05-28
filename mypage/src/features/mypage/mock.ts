import type { MyPageResponse } from './types'

export function createMockMyPageResponse(): MyPageResponse {
	// 未登録状態の表示を確認したいときは、ここを一時的に次へ変えます。
	// return {
	//     status: 'not_registered',
	//     viewer: {
	//         youtubeChannelId: 'UCxxxxxxxxxxxxxxxxxxxxxx',
	//         displayName: 'サンプルユーザー',
	//         profileImageUrl: 'https://placehold.co/96x96',
	//     },
	// }
	return {
		status: 'ok',
		viewer: {
			youtubeChannelId: 'UCxxxxxxxxxxxxxxxxxxxxxx',
			displayName: 'サンプルユーザー',
			profileImageUrl: 'https://placehold.co/96x96',
		},
		stats: {
			dailyWorkSec: 2 * 60 * 60 + 35 * 60,
			cumulativeWorkSec: 1234 * 60 * 60 + 20 * 60,
		},
		currentSeat: {
			seatId: 12,
			isMemberSeat: false,
			state: 'work',
			workName: 'Go API 実装',
			breakWorkName: '',
			startedAt: new Date().toISOString(),
			until: new Date(Date.now() + 40 * 60 * 1000).toISOString(),
		},
	}
}
