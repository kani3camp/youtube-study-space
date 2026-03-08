import type { Meta, StoryObj } from '@storybook/react'
import React from 'react'

import SeatBox, { type SeatProps } from '../components/SeatBox'
import { SeatState } from '../components/SeatsPage'
import type { Seat } from '../types/api'

/** ストーリー用。Map は Controls でシリアライズされないため decorator で注入する */
const defaultMenuImageMap = new Map<string, string>([
	['coffee', '/images/menu_default.svg'],
])

const meta = {
	title: 'SeatBox',
	component: SeatBox,
	parameters: {},
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

export const Vacant: Story = {
	name: '一般席 空席',
	args: {
		globalSeatId: 33,
		isUsed: false,
		memberOnly: false,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		seatPosition,
		seatShape: generalSeatShape,
		roomShape,
	} as SeatProps,
}

export const VacantMember: Story = {
	name: 'メンバー席 空席',
	args: {
		globalSeatId: 33,
		isUsed: false,
		memberOnly: true,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		seatPosition,
		seatShape: memberSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUse: Story = {
	name: '一般席',
	args: {
		globalSeatId: 12,
		isUsed: true,
		memberOnly: false,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 5,
				color_gradient_enabled: false,
			},
			state: SeatState.Work,
		} as Seat,
		seatPosition,
		seatShape: generalSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseLongWorkName: Story = {
	name: '一般席 長い作業名（2行）',
	args: {
		globalSeatId: 12,
		isUsed: true,
		memberOnly: false,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: 'プログラミングの勉強をしています',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 0,
				color_gradient_enabled: false,
			},
			state: SeatState.Work,
		} as Seat,
		seatPosition,
		seatShape: generalSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseMember: Story = {
	name: 'メンバー席',
	args: {
		globalSeatId: 5,
		isUsed: true,
		memberOnly: true,
		hoursRemaining: 0,
		minutesRemaining: 10,
		hoursElapsed: 1,
		minutesElapsed: 3,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容は２行までOK',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 5,
				color_gradient_enabled: false,
			},
			menu_code: '',
			state: SeatState.Work,
			user_profile_image_url: '/images/sample_profile.svg',
		} as Seat,
		seatPosition,
		seatShape: memberSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseMemberNoWorkName: Story = {
	name: 'メンバー席 作業名なし',
	args: {
		globalSeatId: 5,
		isUsed: true,
		memberOnly: true,
		hoursRemaining: 2,
		minutesRemaining: 125,
		hoursElapsed: 0,
		minutesElapsed: 35,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '',
			break_work_name: '',
			appearance: {
				color_code1: '#e0a030',
				color_code2: '#ff6600',
				num_stars: 3,
				color_gradient_enabled: false,
			},
			menu_code: '',
			state: SeatState.Work,
			user_profile_image_url: '/images/sample_profile.svg',
		} as Seat,
		seatPosition,
		seatShape: memberSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseWithMenu: Story = {
	name: '一般席 メニューアイテム',
	args: {
		globalSeatId: 12,
		isUsed: true,
		memberOnly: false,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 5,
				color_gradient_enabled: false,
			},
			menu_code: 'coffee',
			state: SeatState.Work,
		} as Seat,
		seatPosition,
		seatShape: generalSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseMemberWithMenu: Story = {
	name: 'メンバー席 メニューアイテム',
	args: {
		globalSeatId: 5,
		isUsed: true,
		memberOnly: true,
		hoursRemaining: 0,
		minutesRemaining: 10,
		hoursElapsed: 1,
		minutesElapsed: 3,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容は２行までOK',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 5,
				color_gradient_enabled: false,
			},
			menu_code: 'coffee',
			state: SeatState.Work,
			user_profile_image_url: '/images/sample_profile.svg',
		} as Seat,
		seatPosition,
		seatShape: memberSeatShape,
		roomShape,
	} as SeatProps,
}

export const InBreak: Story = {
	name: '一般席 休憩中',
	args: {
		globalSeatId: 12,
		isUsed: true,
		memberOnly: false,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容',
			break_work_name: '休憩内容',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 5,
				color_gradient_enabled: false,
			},
			menu_code: 'coffee',
			state: SeatState.Break,
		} as Seat,
		seatPosition,
		seatShape: generalSeatShape,
		roomShape,
	} as SeatProps,
}

export const InBreakMember: Story = {
	name: 'メンバー席 休憩中',
	args: {
		globalSeatId: 5,
		isUsed: true,
		memberOnly: true,
		hoursRemaining: 0,
		minutesRemaining: 25,
		hoursElapsed: 0,
		minutesElapsed: 45,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '作業内容',
			break_work_name: '散歩中',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 2,
				color_gradient_enabled: false,
			},
			menu_code: 'coffee',
			state: SeatState.Break,
			user_profile_image_url: '/images/sample_profile.svg',
		} as Seat,
		seatPosition,
		seatShape: memberSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseGradient: Story = {
	name: '一般席 グラデーション',
	args: {
		globalSeatId: 12,
		isUsed: true,
		memberOnly: false,
		seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '勉強中',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 0,
				color_gradient_enabled: true,
			},
			state: SeatState.Work,
		} as Seat,
		seatPosition,
		seatShape: generalSeatShape,
		roomShape,
	} as SeatProps,
}

export const InUseMemberGradient: Story = {
	name: 'メンバー席 グラデーション',
	args: {
		globalSeatId: 5,
		isUsed: true,
		memberOnly: true,
		hoursRemaining: 1,
		minutesRemaining: 80,
		hoursElapsed: 2,
		minutesElapsed: 130,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		processingSeat: {
			seat_id: 1,
			user_id: 'user1',
			user_display_name: 'ユーザー名',
			work_name: '数学の勉強',
			break_work_name: '',
			appearance: {
				color_code1: '#5bd27d',
				color_code2: '#008cff',
				num_stars: 10,
				color_gradient_enabled: true,
			},
			menu_code: '',
			state: SeatState.Work,
			user_profile_image_url: '/images/sample_profile.svg',
		} as Seat,
		seatPosition,
		seatShape: memberSeatShape,
		roomShape,
	} as SeatProps,
}
