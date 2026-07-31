import type { Viewer } from '../types'

type ChannelCardProps = {
	viewer: Viewer
}

export function ChannelCard({ viewer }: ChannelCardProps) {
	return (
		<section className="card channelCard">
			<img className="avatar" src={viewer.profileImageUrl} alt="" />
			<div>
				<p className="cardLabel">ログイン中のチャンネル</p>
				<h2 className="channelName">{viewer.displayName}</h2>
				<p className="mutedText">{viewer.youtubeChannelId}</p>
			</div>
		</section>
	)
}
