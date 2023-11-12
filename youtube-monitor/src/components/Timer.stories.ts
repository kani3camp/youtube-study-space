import type { Meta, StoryObj } from '@storybook/react'
import Timer from './Timer'

const meta: Meta<typeof Timer> = {
    title: 'Timer',
    component: Timer,
    parameters: {
        // More on how to position stories at: https://storybook.js.org/docs/react/configure/story-layout
        layout: 'fullscreen',
    },
}

export default meta
type Story = StoryObj<typeof Timer>

export const Default: Story = {
    args: {
        // user: {
        //     name: 'Jane Doe',
        // },
    },
}
