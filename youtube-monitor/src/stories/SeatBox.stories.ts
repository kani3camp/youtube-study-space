import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { Timestamp } from 'firebase/firestore'
import React from 'react'

import SeatBox, { type SeatProps } from '../components/SeatBox'
import { SeatState } from '../components/SeatsPage'
import type { Seat } from '../types/api'

const defaultMenuImageMap = new Map<string, string>([
	['coffee', '/images/menu_default.svg'],
])

type SeatStoryProps = Omit<SeatProps, 'menuImageMap'>

const SeatBoxStory = (props: SeatStoryProps) =>
	React.createElement(SeatBox, {
		...props,
		menuImageMap: defaultMenuImageMap,
	})

const meta = {
	title: 'SeatBox',
	component: SeatBoxStory,
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
} satisfies Meta<typeof SeatBoxStory>

export default meta
type Story = StoryObj<typeof meta>

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

const createSeat = (overrides: Partial<Seat> = {}): Seat => {
	const now = Timestamp.now()

	const baseSeat = {
		seat_id: 1,
		user_id: 'user1',
		user_display_name: 'ユーザー名',
		work_name: '作業内容',
		break_work_name: '',
		entered_at: now,
		until: now,
		appearance: {
			color_code1: '#5BD27D',
			color_code2: '#008CFF',
			num_stars: 5,
			color_gradient_enabled: false,
		},
		menu_code: '',
		state: SeatState.Work,
		current_state_started_at: now,
		current_state_until: now,
		cumulative_work_sec: 60 * 60,
		daily_cumulative_work_sec: 60 * 60,
		user_profile_image_url: '/images/sample_profile.svg',
	} satisfies Seat

	return {
		...baseSeat,
		...overrides,
	}
}

const createBaseArgs = (
	overrides: Partial<SeatStoryProps> = {},
): SeatStoryProps => ({
	globalSeatId: 123,
	isUsed: false,
	memberOnly: false,
	hoursRemaining: 0,
	minutesRemaining: 10,
	hoursElapsed: 1,
	minutesElapsed: 3,
	seatFontSizePx: GENERAL_SEAT_FONT_SIZE,
	processingSeat: createSeat(),
	seatPosition,
	seatShape: generalSeatShape,
	roomShape,
	...overrides,
})

export const Vacant: Story = {
	name: '一般席 空席',
	args: createBaseArgs(),
}

export const InUseNoWorkName: Story = {
	name: '一般席 作業名なし',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat({
			work_name: '',
		}),
	}),
}

export const InUse: Story = {
	name: '一般席',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat(),
	}),
}

export const InUseWithMenu: Story = {
	name: '一般席 メニューアイテム',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat({
			menu_code: 'coffee',
		}),
	}),
}

export const InUseWithLongWorkName: Story = {
	name: '一般席 長い作業内容',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat({
			work_name:
				'作業内容が長い場合はある程度までフォントサイズが自動調整されることを確認する',
		}),
	}),
}

export const InUseWithGreetingWorkName: Story = {
	name: '一般席 おかえりなさいませ',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat({
			work_name: 'おかえりなさいませ👋',
		}),
	}),
}

export const InUseInBreak: Story = {
	name: '一般席 休憩中',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat({
			work_name: '実装タスク',
			break_work_name: 'コーヒー休憩',
			state: SeatState.Break,
		}),
	}),
}

export const InUseWithRankGradient: Story = {
	name: '一般席 ランク表示グラデーション',
	args: createBaseArgs({
		isUsed: true,
		processingSeat: createSeat({
			appearance: {
				color_code1: '#5BD27D',
				color_code2: '#008CFF',
				num_stars: 5,
				color_gradient_enabled: true,
			},
		}),
	}),
}

export const VacantMember: Story = {
	name: 'メンバー席 空席',
	args: createBaseArgs({
		memberOnly: true,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		seatShape: memberSeatShape,
	}),
}

export const InUseNoWorkNameMember: Story = {
	name: 'メンバー席 作業名なし',
	args: createBaseArgs({
		isUsed: true,
		memberOnly: true,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		seatShape: memberSeatShape,
		processingSeat: createSeat({
			work_name: '',
		}),
	}),
}

export const InUseMember: Story = {
	name: 'メンバー席',
	args: createBaseArgs({
		isUsed: true,
		memberOnly: true,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		seatShape: memberSeatShape,
		processingSeat: createSeat({}),
	}),
}

export const InUseMemberWithTwoLinesWorkName: Story = {
	name: 'メンバー席 作業内容2行まで表示',
	args: createBaseArgs({
		isUsed: true,
		memberOnly: true,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		seatShape: memberSeatShape,
		processingSeat: createSeat({
			work_name: '作業内容は2行まで表示',
		}),
	}),
}

export const InUseMemberWithMenu: Story = {
	name: 'メンバー席 メニューアイテム',
	args: createBaseArgs({
		isUsed: true,
		memberOnly: true,
		seatFontSizePx: MEMBER_SEAT_FONT_SIZE,
		seatShape: memberSeatShape,
		processingSeat: createSeat({
			menu_code: 'coffee',
			work_name: '執筆と資料確認',
		}),
	}),
}

export const InUseMemberInBreak: Story = {
	name: 'メンバー席 休憩中',
	args: createBaseArgs({
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
}
export const InUseMemberWithRankGradient: Story = {
	name: 'メンバー席 ランク表示グラデーション',
	args: createBaseArgs({
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
}
