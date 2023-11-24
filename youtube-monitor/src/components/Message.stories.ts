import type { Meta, StoryObj } from '@storybook/react'
import Message from './Message'

const meta: Meta<typeof Message> = {
    title: 'Message',
    component: Message,
}

export default meta
type Story = StoryObj<typeof Message>

export const Default: Story = {
    args: {
        currentPageIndex: 0,
        currentPagesLength: 3,
        currentPageIsMember: false,
        seats: [],
    },
}
