import { Link } from '@tanstack/react-router'

import { formatDuration } from '../../../lib/format'
import type { MyPageResponse } from '../types'
import { ChannelCard } from './ChannelCard'
import { CurrentSeatCard } from './CurrentSeatCard'
import { NotRegisteredPanel } from './NotRegisteredPanel'
import { SummaryCard } from './SummaryCard'

type MyPageViewProps = {
	data: MyPageResponse
}

export function MyPageView({ data }: MyPageViewProps) {
	if (data.status === 'not_registered') {
		return (
			<div className="cardStack">
				<ChannelCard viewer={data.viewer} />
				<NotRegisteredPanel />
				<div className="footerActions">
					<Link to="/logout">ログアウト</Link>
				</div>
			</div>
		)
	}

	return (
		<div className="cardStack">
			<ChannelCard viewer={data.viewer} />

			<div className="summaryGrid">
				<SummaryCard
					label="今日の作業時間"
					value={formatDuration(data.stats.dailyWorkSec)}
				/>
				<SummaryCard
					label="累計作業時間"
					value={formatDuration(data.stats.cumulativeWorkSec)}
				/>
			</div>

			<CurrentSeatCard currentSeat={data.currentSeat} />

			<div className="footerActions">
				<Link to="/logout">ログアウト</Link>
			</div>
		</div>
	)
}
