export const maintenanceAnnouncementURLs = {
	development: 'https://www.youtube.com/channel/UChBj4xZePvWyGmTKct5FBcg/posts',
	production: 'https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/posts',
} as const

// These are reviewed public destinations, not runtime state or secrets. The
// existing public Firebase project ID selects the environment; an unknown
// project must not silently send users to either channel.
export function maintenanceAnnouncementURLForProject(
	projectID: unknown,
): string | null {
	if (projectID === 'test-youtube-study-space')
		return maintenanceAnnouncementURLs.development
	if (projectID === 'youtube-study-space')
		return maintenanceAnnouncementURLs.production
	return null
}
