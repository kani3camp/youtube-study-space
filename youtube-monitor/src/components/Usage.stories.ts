import type { Meta, StoryObj } from '@storybook/react'
import Usage from './Usage'

const meta: Meta<typeof Usage> = {
    title: 'Usage',
    component: Usage,
}

export default meta
type Story = StoryObj<typeof Usage>

export const Default: Story = {
    args: {},
}
