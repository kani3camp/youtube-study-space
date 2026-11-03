import { describe, expect, it } from 'vitest'
import { youtubeChannelURLsForProject } from './youtube-channel'

describe('YouTube channel destinations', () => {
	it.each([
		['test-youtube-study-space', 'UChBj4xZePvWyGmTKct5FBcg'],
		['youtube-study-space', 'UCXuD2XmPTdpVy7zmwbFVZWg'],
	])('uses the same channel for live and posts in %s', (projectID, channelID) => {
		expect(youtubeChannelURLsForProject(projectID)).toEqual({
			live: `https://www.youtube.com/channel/${channelID}/live`,
			posts: `https://www.youtube.com/channel/${channelID}/posts`,
		})
	})
	it.each([
		undefined,
		null,
		'',
		'other-project',
		'development',
		'production',
	])('does not fall back to a channel for %s', (projectID) =>
		expect(youtubeChannelURLsForProject(projectID)).toBeNull())
})
