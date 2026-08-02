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
import { mainContent } from '../../styles/MainContent.styles'
import AmbientFrame from '../AmbientFrame'
import BackgroundImage from '../BackgroundImage'
import BgmPlayer from '../BgmPlayer'
import CenterLoading from '../CenterLoading'
import Clock from '../Clock'
import ColorBar from '../ColorBar'
import MenuDisplay from '../MenuDisplay'
import Message from '../Message'
import TickerBoard from '../TickerBoard'
import Timer from '../Timer'
import Usage from '../Usage'
import RoomStage from './RoomStage'

const roomViewport = {
	width: Constants.screenWidth - Constants.sideBarWidth,
	height: Constants.screenHeight - Constants.messageBarHeight,
}

const HorizontalMonitor: FC = () => {
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
		enabled: true,
		ready: monitorData.seatCapacityControlReady,
		generalSeats: monitorData.generalSeats,
		memberSeats: monitorData.memberSeats,
		systemConstants: monitorData.systemConstants,
	})

	const currentPage = pages[currentPageIndex]

	return (
		<div
			css={themedRoot}
			data-time-theme={timeTheme}
			data-text-tone={textTone}
			data-contrast-bridge={contrastBridge ? 'true' : undefined}
			style={{
				height: Constants.screenHeight,
				width: Constants.screenWidth,
				margin: 0,
				position: 'relative',
				overflow: 'hidden',
			}}
		>
			<BackgroundImage />
			<BgmPlayer />
			<Clock />
			<Usage />
			<MenuDisplay menuItems={monitorData.menuItems} />
			<Timer />
			<ColorBar />

			<div css={mainContent}>
				{pages.length > 0 ? (
					<>
						<RoomStage
							pages={pages}
							currentPageIndex={currentPageIndex}
							menuImageMap={monitorData.menuImageMap}
							viewport={roomViewport}
						/>
						<Message
							currentPageIndex={currentPageIndex}
							currentPagesLength={pages.length}
							currentPageIsMember={currentPage?.memberOnly ?? false}
							seats={monitorData.generalSeats.concat(monitorData.memberSeats)}
						/>
						<TickerBoard workNameTrend={monitorData.workNameTrend} />
					</>
				) : (
					<CenterLoading />
				)}
			</div>
			<AmbientFrame />
		</div>
	)
}

export default HorizontalMonitor
