import { describe, expect, it } from 'vitest'
import {
	maintenanceAnnouncementURLForProject,
	maintenanceAnnouncementURLs,
} from './maintenance-announcement'

describe('maintenance announcement destinations', () => {
	it('selects the reviewed DEV and PROD channels from the public project ID', () => {
		expect(maintenanceAnnouncementURLs).toEqual({
			development:
				'https://www.youtube.com/channel/UChBj4xZePvWyGmTKct5FBcg/posts',
			production:
				'https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/posts',
		})
		expect(
			maintenanceAnnouncementURLForProject('test-youtube-study-space'),
		).toBe(maintenanceAnnouncementURLs.development)
		expect(maintenanceAnnouncementURLForProject('youtube-study-space')).toBe(
			maintenanceAnnouncementURLs.production,
		)
	})

	it.each([
		undefined,
		null,
		'',
		'demo-youtube-study-space-ci',
		'other-project',
	])('does not fall back across environments for %s', (projectID) => {
		expect(maintenanceAnnouncementURLForProject(projectID)).toBeNull()
	})
})
