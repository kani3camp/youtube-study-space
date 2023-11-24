import type { Meta, StoryObj } from '@storybook/react'
import CenterLoading from './CenterLoading'

const meta: Meta<typeof CenterLoading> = {
    title: 'CenterLoading',
    component: CenterLoading,
}

export default meta
type Story = StoryObj<typeof CenterLoading>

export const Default: Story = {
    args: {},
}
