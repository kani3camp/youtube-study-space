import { describe, expect, it } from 'vitest'
import {
	maintenanceAnnouncementURLForProject,
	maintenanceAnnouncementURLs,
} from './maintenance-announcement'

describe('maintenance announcement destinations', () => {
	it('selects the reviewed DEV and PROD channels from the public project ID', () => {
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

	it('keeps both destinations canonical, public YouTube posts URLs', () => {
		for (const destination of Object.values(maintenanceAnnouncementURLs)) {
			const url = new URL(destination)
			expect(url.protocol).toBe('https:')
			expect(url.hostname).toBe('www.youtube.com')
			expect(url.pathname).toMatch(/^\/channel\/UC[A-Za-z0-9_-]+\/posts$/)
			expect(url.search).toBe('')
			expect(url.hash).toBe('')
			expect(url.username).toBe('')
			expect(url.password).toBe('')
		}
	})
})
