import type { Meta, StoryObj } from '@storybook/react'

import Clock from '../components/Clock'

const meta = {
	title: 'Clock',
	component: Clock,
	parameters: {},
	tags: ['autodocs'],
	argTypes: {},
} satisfies Meta<typeof Clock>

export default meta
type Story = StoryObj<typeof Clock>

export const Default: Story = {
	name: 'デフォルト',
	args: {
		time: new Date(),
	},
}
