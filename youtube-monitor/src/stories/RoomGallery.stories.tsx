import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import RoomGallery from '../dev/RoomGallery'

const meta = {
	title: 'Development / Room Gallery',
	component: RoomGallery,
	parameters: {
		layout: 'fullscreen',
	},
	tags: ['autodocs'],
} satisfies Meta<typeof RoomGallery>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
