import type { Meta, StoryObj } from '@storybook/react'

import SeatBox, { type SeatProps } from '../components/SeatBox'
import { Seat } from '../types/api'
import { Timestamp } from 'firebase/firestore'
import { SeatState } from '../components/SeatsPage'

const meta = {
    title: 'SeatBox',
    component: SeatBox,
    parameters: {},
    tags: [],
    argTypes: {},
} satisfies Meta<typeof SeatBox>

export default meta
type Story = StoryObj<typeof SeatBox>

export const Vacant: Story = {
    args: {
        globalSeatId: 1,
        isUsed: false,
        memberOnly: false,
        seatFontSizePx: 20,
        seatPosition: {
            x: 3,
            y: 3,
            rotate: 0,
        },
        seatShape: {
            widthPercent: (140 * 100) / 1520,
            heightPercent: (100 * 100) / 1000,
        },
        roomShape: {
            widthPx: 1520,
            heightPx: 1000,
        },
    } as SeatProps,
}

export const InUse: Story = {
    args: {
        globalSeatId: 1,
        isUsed: true,
        memberOnly: false,
        hoursRemaining: 0,
        minutesRemaining: 10,
        hoursElapsed: 1,
        minutesElapsed: 3,
        seatFontSizePx: 20,
        processingSeat: {
            seat_id: 1,
            user_id: 'user1',
            user_display_name: 'ユーザー名',
            work_name: '作業内容',
            break_work_name: '',
            entered_at: Timestamp.now(),
            until: Timestamp.now(),
            appearance: {
                color_code1: '#5bd27d',
                color_code2: '#008cff',
                num_stars: 5,
                color_gradient_enabled: false,
            },
            menu_code: '',
            state: SeatState.Work,
            current_state_started_at: Timestamp.now(),
            current_state_until: Timestamp.now(),
            cumulative_work_sec: 0,
            daily_cumulative_work_sec: 0,
            user_profile_image_url:
                'https://yt3.ggpht.com/exjUpNy_ufpwI6oAdz-UVAp17C67z9ObW8j_QK-wMlXVEI4eXq0736r3VeWf6Kyd5zjljD1PozQ=s108-c-k-c0x00ffffff-no-rj',
        } as Seat,
        seatPosition: {
            x: 3,
            y: 3,
            rotate: 0,
        },
        seatShape: {
            widthPercent: (140 * 100) / 1520,
            heightPercent: (100 * 100) / 1000,
        },
        roomShape: {
            widthPx: 1520,
            heightPx: 1000,
        },
    } as SeatProps,
}

export const InUseMember: Story = {
    args: {
        globalSeatId: 1,
        isUsed: true,
        memberOnly: true,
        hoursRemaining: 0,
        minutesRemaining: 10,
        hoursElapsed: 1,
        minutesElapsed: 3,
        seatFontSizePx: 20,
        processingSeat: {
            seat_id: 1,
            user_id: 'user1',
            user_display_name: 'ユーザー名',
            work_name: '作業内容',
            break_work_name: '',
            entered_at: Timestamp.now(),
            until: Timestamp.now(),
            appearance: {
                color_code1: '#5bd27d',
                color_code2: '#008cff',
                num_stars: 5,
                color_gradient_enabled: false,
            },
            menu_code: '',
            state: SeatState.Work,
            current_state_started_at: Timestamp.now(),
            current_state_until: Timestamp.now(),
            cumulative_work_sec: 0,
            daily_cumulative_work_sec: 0,
            user_profile_image_url:
                'https://yt3.ggpht.com/exjUpNy_ufpwI6oAdz-UVAp17C67z9ObW8j_QK-wMlXVEI4eXq0736r3VeWf6Kyd5zjljD1PozQ=s108-c-k-c0x00ffffff-no-rj',
        } as Seat,
        seatPosition: {
            x: 3,
            y: 3,
            rotate: 0,
        },
        seatShape: {
            widthPercent: (140 * 100) / 1520,
            heightPercent: (100 * 100) / 1000,
        },
        roomShape: {
            widthPx: 1520,
            heightPx: 1000,
        },
    } as SeatProps,
}
