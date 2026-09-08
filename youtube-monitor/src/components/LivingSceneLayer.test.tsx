import { act, render, screen } from '@testing-library/react'
import { livingSceneRuntime } from '../lib/living-scene-runtime'
import LivingSceneLayer from './LivingSceneLayer'

jest.mock('../lib/living-scene-runtime', () => ({
	livingSceneRuntime: {
		activate: jest.fn(() => Promise.resolve(true)),
		deactivate: jest.fn(),
	},
}))

const runtimeMock = livingSceneRuntime as jest.Mocked<typeof livingSceneRuntime>

describe('LivingSceneLayer', () => {
	beforeEach(() => {
		jest.clearAllMocks()
	})

	test('activates the shared runtime only while the room page is visible', async () => {
		const { rerender } = render(
			<LivingSceneLayer active={false} width={1520} height={900} />,
		)
		const host = screen.getByTestId('living-scene-host')

		expect(runtimeMock.activate).not.toHaveBeenCalled()
		expect(runtimeMock.deactivate).toHaveBeenCalledWith(host)

		await act(async () => {
			rerender(<LivingSceneLayer active={true} width={1520} height={900} />)
		})

		expect(runtimeMock.activate).toHaveBeenCalledWith({
			host,
			width: 1520,
			height: 900,
		})

		rerender(<LivingSceneLayer active={false} width={1520} height={900} />)
		expect(runtimeMock.deactivate).toHaveBeenCalledWith(host)
	})
})
