import type { Meta, StoryObj } from '@storybook/react'
import Clock from './Clock'

const meta: Meta<typeof Clock> = {
    title: 'Clock',
    component: Clock,
}

export default meta
type Story = StoryObj<typeof Clock>

export const Default: Story = {
    args: {},
}
