import { useRouter } from 'next/router'
import type { FC } from 'react'
import { useTimeTheme } from '../../hooks/use-time-theme'
import useMonitorData from '../../hooks/useMonitorData'
import useRoomPages from '../../hooks/useRoomPages'
import useSeatCapacityController from '../../hooks/useSeatCapacityController'
import {
	getFixedPageIndex,
	useSynchronizedPage,
} from '../../hooks/useSynchronizedPage'
import { Constants } from '../../lib/constants'
import { themedRoot } from '../../styles/AmbientFrame.styles'
import * as styles from '../../styles/VerticalMonitor.styles'
import AmbientFrame from '../AmbientFrame'
import BackgroundImage from '../BackgroundImage'
import CenterLoading from '../CenterLoading'
import Clock from '../Clock'
import Message from '../Message'
import TickerBoard from '../TickerBoard'
import Timer from '../Timer'
import Usage from '../Usage'
import RoomStage from './RoomStage'

const roomViewport = {
	width: 1032,
	height: 810,
}

const VerticalMonitor: FC = () => {
	const router = useRouter()
	const { timeTheme, textTone, contrastBridge } = useTimeTheme()
	const monitorData = useMonitorData()
	const pages = useRoomPages(monitorData)
	const fixedPageIndex = getFixedPageIndex(router.query.page, pages.length)
	const currentPageIndex = useSynchronizedPage({
		pageCount: pages.length,
		intervalMs: Constants.pagingIntervalSeconds * 1000,
		fixedPageIndex,
	})

	useSeatCapacityController({
		enabled: false,
		ready: false,
		generalSeats: monitorData.generalSeats,
		memberSeats: monitorData.memberSeats,
		systemConstants: monitorData.systemConstants,
	})

	const currentPage = pages[currentPageIndex]
	const allSeats = monitorData.generalSeats.concat(monitorData.memberSeats)

	return (
		<div
			css={themedRoot}
			data-time-theme={timeTheme}
			data-text-tone={textTone}
			data-contrast-bridge={contrastBridge ? 'true' : undefined}
			style={{
				height: Constants.verticalScreenHeight,
				width: Constants.verticalScreenWidth,
				margin: 0,
				position: 'relative',
				overflow: 'hidden',
			}}
		>
			<BackgroundImage
				width={Constants.verticalScreenWidth}
				height={Constants.verticalScreenHeight}
			/>
			<Clock
				style={{
					top: 24,
					left: 24,
					right: 'auto',
					width: 430,
					height: 112,
				}}
			/>
			{pages.length > 0 && (
				<Message
					currentPageIndex={currentPageIndex}
					currentPagesLength={pages.length}
					currentPageIsMember={currentPage?.memberOnly ?? false}
					seats={allSeats}
					style={{
						top: 24,
						left: 478,
						right: 'auto',
						bottom: 'auto',
						width: 478,
						height: 112,
					}}
				/>
			)}
			<RoomStage
				pages={pages}
				currentPageIndex={currentPageIndex}
				menuImageMap={monitorData.menuImageMap}
				viewport={roomViewport}
				style={{ top: 210, left: 24, position: 'absolute' }}
			/>
			<Timer
				style={{
					top: 1050,
					left: 24,
					right: 'auto',
					bottom: 'auto',
					width: 360,
					height: 250,
				}}
			/>
			<TickerBoard
				workNameTrend={monitorData.workNameTrend}
				style={{
					top: 1050,
					left: 408,
					right: 'auto',
					bottom: 'auto',
					width: 540,
					height: 250,
				}}
			/>
			<Usage
				style={{
					top: 1330,
					left: 24,
					right: 'auto',
					bottom: 'auto',
					width: 912,
					height: 220,
				}}
			/>
			<div css={styles.footer}>
				<h2 css={styles.footerTitle}>静かに、一緒に作業できます</h2>
				<p css={styles.footerText}>
					カメラ・マイク不要。YouTubeのチャットから自由に参加・退室できます。
				</p>
			</div>
			{pages.length === 0 && <CenterLoading />}
			<AmbientFrame vertical />
		</div>
	)
}

export default VerticalMonitor
