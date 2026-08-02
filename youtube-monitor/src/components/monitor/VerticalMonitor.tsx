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
import { verticalLayout } from '../../lib/vertical-layout'
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
				<header css={styles.headerGrid}>
					<section css={styles.verticalPanel}>
						<Clock variant="vertical" />
					</section>
					<section css={styles.verticalPanel}>
						{pages.length > 0 && (
							<Message
								currentPageIndex={currentPageIndex}
								currentPagesLength={pages.length}
								currentPageIsMember={currentPage?.memberOnly ?? false}
								seats={allSeats}
								variant="vertical"
							/>
						)}
					</section>
				</header>
				<section css={styles.roomSection}>
					<RoomStage
						pages={pages}
						currentPageIndex={currentPageIndex}
						menuImageMap={monitorData.menuImageMap}
						viewport={verticalLayout.roomViewport}
					/>
				</section>
				<section css={styles.informationGrid}>
					<div css={[styles.verticalPanel, styles.timerPanel]}>
						<Timer variant="vertical" />
					</div>
					<div css={styles.informationDetails}>
						<div css={[styles.verticalPanel, styles.usagePanel]}>
							<Usage variant="vertical" />
						</div>
						<div css={[styles.verticalPanel, styles.tickerPanel]}>
							<TickerBoard
								workNameTrend={monitorData.workNameTrend}
								variant="vertical"
							/>
						</div>
					</div>
				</section>
				<section css={styles.taglineSection}>
					<h2 css={styles.taglineTitle}>静かに、一緒に作業できます</h2>
				</section>
			</main>
			{pages.length === 0 && <CenterLoading />}
			<AmbientFrame vertical />
		</div>
	)
}

export default VerticalMonitor
