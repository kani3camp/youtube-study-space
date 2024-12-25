import type { Meta, StoryObj } from '@storybook/react'

import SeatBox, { type SeatProps } from '../components/SeatBox'
import { Seat } from '../types/api'

const meta = {
    title: 'SeatBox',
    component: SeatBox,
    parameters: {},
    tags: [],
    argTypes: {},
} satisfies Meta<typeof SeatBox>

export default meta
type Story = StoryObj<typeof SeatBox>

export const InUse: Story = {
    args: {
        globalSeatId: 1,
        isUsed: true,
        memberOnly: false,
        hoursRemaining: 0,
        minutesRemaining: 10,
        hoursElapsed: 1,
        minutesElapsed: 3,
        seatFontSizePx: 30,
        processingSeat: {
            seat_id: 1,
            user_id: 'user1',
            user_display_name: 'ユーザー名',
            work_name: '',
            break_work_name: '',
            user_profile_image_url: '',
            appearance: {
                color_code1: '#c78181',
                color_code2: '#00ff00',
                num_stars: 5,
                color_gradient_enabled: true,
            },
        } as Seat,
        seatPosition: {
            x: 3,
            y: 3,
            rotate: 0,
        },
        seatShape: {
            widthPercent: 30,
            heightPercent: 50,
        },
        roomShape: {
            widthPx: 10,
            heightPx: 10,
        },
    } as SeatProps,
}
