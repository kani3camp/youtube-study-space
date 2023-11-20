import type { Meta, StoryObj } from '@storybook/react'
import BgmPlayer from './BgmPlayer'

const meta: Meta<typeof BgmPlayer> = {
    title: 'BgmPlayer',
    component: BgmPlayer,
}

export default meta
type Story = StoryObj<typeof BgmPlayer>

export const Default: Story = {
    args: {},
}
