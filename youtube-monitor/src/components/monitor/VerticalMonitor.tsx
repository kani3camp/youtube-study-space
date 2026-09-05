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
import {
	getVerticalRoomViewport,
	verticalLayout,
} from '../../lib/vertical-layout'
import { themedRoot } from '../../styles/AmbientFrame.styles'
import * as styles from '../../styles/VerticalMonitor.styles'
import AmbientFrame from '../AmbientFrame'
import BackgroundImage from '../BackgroundImage'
import CenterLoading from '../CenterLoading'
import Clock from '../Clock'
import Message from '../Message'
import Timer from '../Timer'
import Usage from '../Usage'
import RoomStage from './RoomStage'

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
	const roomViewport = currentPage
		? getVerticalRoomViewport(currentPage.roomLayout.room_shape)
		: undefined

	return (
		<div
			css={[themedRoot, styles.canvas]}
			data-time-theme={timeTheme}
			data-text-tone={textTone}
			data-contrast-bridge={contrastBridge ? 'true' : undefined}
		>
			<BackgroundImage
				width={verticalLayout.canvas.width}
				height={verticalLayout.canvas.height}
				variant="vertical"
			/>
			<div css={styles.backgroundOverlay} />
			<main css={styles.layout}>
				<div css={styles.topSafeArea} data-vertical-section="safe-area" />
				{roomViewport && (
					<>
						<section
							css={styles.roomSection}
							data-vertical-section="room"
							style={{ height: roomViewport.height }}
						>
							<RoomStage
								pages={pages}
								currentPageIndex={currentPageIndex}
								menuImageMap={monitorData.menuImageMap}
								viewport={roomViewport}
							/>
						</section>
						<div css={styles.controlsStack}>
							<section
								css={[styles.verticalPanel, styles.statusPanel]}
								data-vertical-section="status"
							>
								<Clock variant="vertical" />
								<span css={styles.statusDivider} aria-hidden="true" />
								<Message
									currentPageIndex={currentPageIndex}
									currentPagesLength={pages.length}
									currentPageIsMember={currentPage.memberOnly}
									seats={allSeats}
									variant="vertical"
								/>
							</section>
							<section
								css={[styles.verticalPanel, styles.timerPanel]}
								data-vertical-section="timer"
							>
								<Timer variant="vertical" />
							</section>
							<section
								css={[styles.verticalPanel, styles.commandsPanel]}
								data-vertical-section="commands"
							>
								<Usage variant="vertical" />
							</section>
						</div>
					</>
				)}
			</main>
			{pages.length === 0 && <CenterLoading />}
			<AmbientFrame vertical />
		</div>
	)
}

export default VerticalMonitor
