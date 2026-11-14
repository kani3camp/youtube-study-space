import {
	createMemoryHistory,
	RouterContextProvider,
} from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp } from './app'
import { BrowserRuntime } from './runtime'

afterEach(() => vi.unstubAllEnvs())

describe('public home live link', () => {
	it.each([
		['test-youtube-study-space', 'UChBj4xZePvWyGmTKct5FBcg'],
		['youtube-study-space', 'UCXuD2XmPTdpVy7zmwbFVZWg'],
		['other-project', null],
		[undefined, null],
	])('selects the destination at app creation for %s', (projectID, channelID) => {
		vi.stubEnv('VITE_FIREBASE_PROJECT_ID', projectID)
		const request = vi.fn<typeof fetch>()
		const runtime = new BrowserRuntime(
			null,
			{ privacy: '', terms: '' },
			request,
		)
		const router = createApp(runtime, createMemoryHistory())
		const Home = router.routesById['/'].options.component
		if (!Home) throw new Error('Home route missing')
		const html = renderToStaticMarkup(
			<RouterContextProvider router={router}>
				<Home />
			</RouterContextProvider>,
		)
		if (channelID) {
			expect(html).toContain(
				`href="https://www.youtube.com/channel/${channelID}/live" target="_blank" rel="noopener noreferrer"`,
			)
			expect(html.match(/\/live"/g)).toHaveLength(1)
		} else {
			expect(html).not.toContain('YouTubeライブを開く')
			expect(html).not.toContain('/live')
		}
		expect(request).not.toHaveBeenCalled()
	})
})
