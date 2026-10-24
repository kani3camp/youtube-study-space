import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import RuntimeRoomPreview from '../dev/runtime-room-preview/RuntimeRoomPreview'

const meta = {
	title: 'Development/Runtime Room Preview',
	component: RuntimeRoomPreview,
	parameters: {
		layout: 'fullscreen',
		docs: { disable: true },
	},
} satisfies Meta<typeof RuntimeRoomPreview>

export default meta
type Story = StoryObj<typeof meta>

export const Normal: Story = {
	name: '正常系 8席',
	args: { initialFixtureId: 'passing-split-islands' },
}

export const CaloRegression: Story = {
	name: 'Calo current相当 既知問題',
	args: { initialFixtureId: 'calo-current-regression' },
}
