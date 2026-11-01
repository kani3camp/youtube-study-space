function channelURLs(channelID: string) {
	const channelURL = `https://www.youtube.com/channel/${channelID}`
	return { live: `${channelURL}/live`, posts: `${channelURL}/posts` }
}

// レビュー済みの公開チャンネル。ライブとお知らせで対応表を共有する。
export const youtubeChannelURLs = {
	development: channelURLs('UChBj4xZePvWyGmTKct5FBcg'),
	production: channelURLs('UCXuD2XmPTdpVy7zmwbFVZWg'),
} as const

// ViteのDEV/PRODはビルド方式なので、接続先環境の判定には使わない。
// 未知のFirebaseプロジェクトを本番チャンネルへ暗黙に送らない。
export function youtubeChannelURLsForProject(projectID: unknown) {
	if (projectID === 'test-youtube-study-space')
		return youtubeChannelURLs.development
	if (projectID === 'youtube-study-space') return youtubeChannelURLs.production
	return null
}
