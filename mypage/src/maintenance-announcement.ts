export const maintenanceAnnouncementURLs = {
	development: 'https://www.youtube.com/channel/UChBj4xZePvWyGmTKct5FBcg/posts',
	production: 'https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/posts',
} as const

// レビュー済みの公開リンク先であり、実行時状態やシークレットは含まない。
// 既存の公開FirebaseプロジェクトIDで環境を選ぶ。
// 未知のプロジェクトから、どちらかのチャンネルへ暗黙に送らない。
export function maintenanceAnnouncementURLForProject(
	projectID: unknown,
): string | null {
	if (projectID === 'test-youtube-study-space')
		return maintenanceAnnouncementURLs.development
	if (projectID === 'youtube-study-space')
		return maintenanceAnnouncementURLs.production
	return null
}
