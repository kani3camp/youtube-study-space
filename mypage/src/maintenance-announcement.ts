import {
	youtubeChannelURLs,
	youtubeChannelURLsForProject,
} from './youtube-channel'

export const maintenanceAnnouncementURLs = {
	development: youtubeChannelURLs.development.posts,
	production: youtubeChannelURLs.production.posts,
} as const

// レビュー済みの公開リンク先であり、実行時状態やシークレットは含まない。
// 既存の公開FirebaseプロジェクトIDで環境を選ぶ。
// 未知のプロジェクトから、どちらかのチャンネルへ暗黙に送らない。
export function maintenanceAnnouncementURLForProject(
	projectID: unknown,
): string | null {
	return youtubeChannelURLsForProject(projectID)?.posts ?? null
}
