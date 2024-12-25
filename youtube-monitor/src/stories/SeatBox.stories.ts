import type { Meta, StoryObj } from '@storybook/react'

import SeatBox, { type SeatProps } from '../components/SeatBox'
import { Seat } from '../types/api'
import { SeatState } from '../components/SeatsPage'

const meta = {
    title: 'SeatBox',
    component: SeatBox,
    parameters: {},
    tags: ['autodocs'],
    argTypes: {},
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
        globalSeatId: 123,
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
        globalSeatId: 123,
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
        globalSeatId: 123,
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

export const InUseMember: Story = {
    name: 'メンバー席',
    args: {
        globalSeatId: 123,
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
            user_profile_image_url:
                'https://yt3.ggpht.com/exjUpNy_ufpwI6oAdz-UVAp17C67z9ObW8j_QK-wMlXVEI4eXq0736r3VeWf6Kyd5zjljD1PozQ=s108-c-k-c0x00ffffff-no-rj',
        } as Seat,
        seatPosition,
        seatShape: memberSeatShape,
        roomShape,
    } as SeatProps,
}

export const InUseWithMenu: Story = {
    name: '一般席 メニューアイテム',
    args: {
        globalSeatId: 123,
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
        globalSeatId: 123,
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
            user_profile_image_url:
                'https://yt3.ggpht.com/exjUpNy_ufpwI6oAdz-UVAp17C67z9ObW8j_QK-wMlXVEI4eXq0736r3VeWf6Kyd5zjljD1PozQ=s108-c-k-c0x00ffffff-no-rj',
        } as Seat,
        seatPosition,
        seatShape: memberSeatShape,
        roomShape,
    } as SeatProps,
}

export const InBreak: Story = {
    name: '一般席 休憩中',
    args: {
        globalSeatId: 123,
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
