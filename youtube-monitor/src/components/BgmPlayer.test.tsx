import { render } from '@testing-library/react'
import BgmPlayer, { MAX_CONSECUTIVE_BGM_ERRORS } from './BgmPlayer'

vi.mock('@foobar404/wave', () => ({
	Wave: class {
		animations = { Wave: class {} }
		addAnimation() {}
	},
}))
vi.mock('music-metadata', () => ({
	parseBlob: vi.fn(),
	parseWebStream: vi.fn(),
}))

const getRandomBgm = vi.fn<() => Promise<string>>()
vi.mock('../lib/bgm', () => ({
	getCurrentRandomBgm: () => getRandomBgm(),
}))

const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('BgmPlayer', () => {
	beforeEach(() => {
		process.env.NEXT_PUBLIC_DEBUG = 'false'
		getRandomBgm.mockReset()
		// fetch が失敗する状態を作り、audioNext が再生まで進まないようにする
		getRandomBgm.mockResolvedValue('/bgm/a.mp3')
		vi.stubGlobal(
			'fetch',
			vi.fn(() => Promise.reject(new Error('network'))),
		)
		vi.spyOn(console, 'error').mockImplementation(() => undefined)
		vi.spyOn(console, 'log').mockImplementation(() => undefined)
		vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(
			() => undefined,
		)
		vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
	})

	afterEach(() => {
		vi.unstubAllGlobals()
		vi.restoreAllMocks()
	})

	test('does not block the page with alert when a chime fails to load', () => {
		const alert = vi.spyOn(window, 'alert').mockImplementation(() => undefined)
		const { container } = render(<BgmPlayer />)

		for (const audio of container.querySelectorAll('audio')) {
			audio.dispatchEvent(new Event('error'))
		}

		expect(alert).not.toHaveBeenCalled()
	})

	test('stops advancing to the next BGM after consecutive load errors reach the cap', async () => {
		const { container } = render(<BgmPlayer />)
		const bgmAudio = container.querySelector('audio') as HTMLAudioElement
		await flush()
		const initialCalls = getRandomBgm.mock.calls.length

		for (let i = 0; i < MAX_CONSECUTIVE_BGM_ERRORS + 3; i++) {
			bgmAudio.dispatchEvent(new Event('error'))
			await flush()
		}

		expect(getRandomBgm.mock.calls.length - initialCalls).toBe(
			MAX_CONSECUTIVE_BGM_ERRORS - 1,
		)
	})

	test('a successful playback resets the consecutive error count', async () => {
		const { container } = render(<BgmPlayer />)
		const bgmAudio = container.querySelector('audio') as HTMLAudioElement
		await flush()
		const initialCalls = getRandomBgm.mock.calls.length

		for (let i = 0; i < MAX_CONSECUTIVE_BGM_ERRORS - 1; i++) {
			bgmAudio.dispatchEvent(new Event('error'))
		}
		bgmAudio.dispatchEvent(new Event('playing'))
		bgmAudio.dispatchEvent(new Event('error'))
		await flush()

		expect(getRandomBgm.mock.calls.length - initialCalls).toBe(
			MAX_CONSECUTIVE_BGM_ERRORS,
		)
	})

	test('removes the BGM audio listeners on unmount', async () => {
		const { container, unmount } = render(<BgmPlayer />)
		const bgmAudio = container.querySelector('audio') as HTMLAudioElement
		await flush()
		const removed = vi.spyOn(bgmAudio, 'removeEventListener')

		unmount()

		expect(removed.mock.calls.map(([type]) => type).sort()).toEqual([
			'ended',
			'error',
			'playing',
		])
	})
})
