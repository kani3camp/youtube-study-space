import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

import SeatBox, { type SeatProps } from '../components/SeatBox'
import { SeatState } from '../components/SeatsPage'
import type { Seat } from '../types/api'

const defaultMenuImageMap = new Map<string, string>([
	['coffee', '/images/menu_default.svg'],
])

const meta = {
	title: 'SeatBox',
	component: SeatBox,
	parameters: {
		docs: {
			story: {
				inline: false,
				height: 'auto',
			},
		},
	},
	tags: ['autodocs'],
	argTypes: {},
	decorators: [
		(_Story, context) =>
			React.createElement(SeatBox, {
				...context.args,
				menuImageMap: defaultMenuImageMap,
			} as SeatProps),
	],
} satisfies Meta<typeof SeatBox>

export default meta
type Story = StoryObj<typeof SeatBox>

const GENERAL_SEAT_FONT_SIZE = 22.8
const MEMBER_SEAT_FONT_SIZE = 25.84
const seatPosition = {
	x: 3,
	y: 3,
	rotate: 0,
}
const roomShape = {
	widthPx: 1520,
	heightPx: 1000,
}
const generalSeatShape = {
	widthPx: 140,
	heightPx: 100,
}
const memberSeatShape = {
	widthPx: 230,
	heightPx: 150,
}

const createSeat = (overrides: Partial<Seat> = {}): Seat =>
	({
		seat_id: 1,
		user_id: 'user1',
		user_display_name: 'ユーザー名',
		work_name: '作業内容',
		break_work_name: '',
		appearance: {
			color_code1: '#5BD27D',
			color_code2: '#008CFF',
			num_stars: 5,
			color_gradient_enabled: false,
		},
		menu_code: '',
		state: SeatState.Work,
		user_profile_image_url: '/images/sample_profile.svg',
		...overrides,
	}) as Seat

const createBaseArgs = (
	overrides: Partial<SeatProps> = {},
): Partial<SeatProps> => ({
	globalSeatId: 123,
	hoursRemaining: 0,
	minutesRemaining: 10,
	hoursElapsed: 1,
	minutesElapsed: 3,
	seatPosition,
	roomShape,
	...overrides,
})

export const Vacant: Story = {
	name: '一般席 空席',
	args: {
		...createBaseArgs({
			isUsed: false,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
		}),
	} as SeatProps,
}

export const InUseNoWorkName: Story = {
	name: '一般席 作業名なし',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
			processingSeat: createSeat({
				work_name: '',
			}),
		}),
	} as SeatProps,
}

export const InUse: Story = {
	name: '一般席',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
			processingSeat: createSeat(),
		}),
	} as SeatProps,
}

export const InUseWithMenu: Story = {
	name: '一般席 メニューアイテム',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
			processingSeat: createSeat({
				menu_code: 'coffee',
			}),
		}),
	} as SeatProps,
}

export const InUseWithLongWorkName: Story = {
	name: '一般席 長い作業内容',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
			processingSeat: createSeat({
				work_name:
					'作業内容が長い場合はある程度までフォントサイズが自動調整されることを確認する',
			}),
		}),
	} as SeatProps,
}

export const InUseInBreak: Story = {
	name: '一般席 休憩中',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
			processingSeat: createSeat({
				work_name: '実装タスク',
				break_work_name: 'コーヒー休憩',
				state: SeatState.Break,
			}),
		}),
	} as SeatProps,
}

export const InUseWithRankGradient: Story = {
	name: '一般席 ランク表示グラデーション',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: false,
			seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
			seatShape: generalSeatShape,
			processingSeat: createSeat({
				appearance: {
					color_code1: '#5BD27D',
					color_code2: '#008CFF',
					num_stars: 5,
					color_gradient_enabled: true,
				},
			}),
		}),
	} as SeatProps,
}

export const VacantMember: Story = {
	name: 'メンバー席 空席',
	args: {
		...createBaseArgs({
			isUsed: false,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
		}),
	} as SeatProps,
}

export const InUseNoWorkNameMember: Story = {
	name: 'メンバー席 作業名なし',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
			processingSeat: createSeat({
				work_name: '',
			}),
		}),
	} as SeatProps,
}

export const InUseMember: Story = {
	name: 'メンバー席',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
			processingSeat: createSeat({}),
		}),
	} as SeatProps,
}

export const InUseMemberWithTwoLinesWorkName: Story = {
	name: 'メンバー席 作業内容2行まで表示',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
			processingSeat: createSeat({
				work_name: '作業内容は2行まで表示',
			}),
		}),
	} as SeatProps,
}

export const InUseMemberWithMenu: Story = {
	name: 'メンバー席 メニューアイテム',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
			processingSeat: createSeat({
				menu_code: 'coffee',
				work_name: '執筆と資料確認',
			}),
		}),
	} as SeatProps,
}

export const InUseMemberInBreak: Story = {
	name: 'メンバー席 休憩中',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
			processingSeat: createSeat({
				work_name: '実装タスク',
				break_work_name: 'コーヒー休憩',
				state: SeatState.Break,
			}),
		}),
	} as SeatProps,
}
export const InUseMemberWithRankGradient: Story = {
	name: 'メンバー席 ランク表示グラデーション',
	args: {
		...createBaseArgs({
			isUsed: true,
			memberOnly: true,
			seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
			seatShape: memberSeatShape,
			processingSeat: createSeat({
				appearance: {
					color_code1: '#5BD27D',
					color_code2: '#008CFF',
					num_stars: 5,
					color_gradient_enabled: true,
				},
			}),
		}),
	} as SeatProps,
}
