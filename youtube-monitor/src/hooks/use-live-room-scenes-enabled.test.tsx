import { TextEncoder } from 'node:util'
import { render, screen } from '@testing-library/react'
import type { NextRouter } from 'next/router'
import { useRouter } from 'next/router'
import { useLiveRoomScenesEnabled } from './use-live-room-scenes-enabled'

jest.mock('next/router', () => ({
	useRouter: jest.fn(),
}))

jest.mock('../lib/constants', () => ({
	DEBUG: true,
}))

const useRouterMock = useRouter as jest.MockedFunction<typeof useRouter>

const Probe = () => {
	const enabled = useLiveRoomScenesEnabled()
	return <span data-testid="status">{enabled ? 'on' : 'off'}</span>
}

function createRouter(query: NextRouter['query']): NextRouter {
	return {
		isReady: true,
		query,
	} as NextRouter
}

describe('useLiveRoomScenesEnabled', () => {
	beforeEach(() => {
		useRouterMock.mockReset()
	})

	test('keeps the server render hydration-safe even when debug force-on is present', async () => {
		useRouterMock.mockReturnValue(createRouter({ liveScenes: 'on' }))
		Object.assign(globalThis, { TextEncoder })
		const { renderToString } = await import('react-dom/server')

		const html = renderToString(<Probe />)

		expect(html).toContain('off')
		expect(html).not.toContain('>on<')
	})

	test('enables after mount when debug force-on is present', () => {
		useRouterMock.mockReturnValue(createRouter({ liveScenes: 'on' }))

		render(<Probe />)

		expect(screen.getByTestId('status')).toHaveTextContent('on')
	})

	test('keeps the runtime kill switch disabled after mount', () => {
		useRouterMock.mockReturnValue(createRouter({ liveScenes: 'off' }))

		render(<Probe />)

		expect(screen.getByTestId('status')).toHaveTextContent('off')
	})
})
