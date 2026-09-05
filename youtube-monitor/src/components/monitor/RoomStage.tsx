import type { FC } from 'react'
import SeatsPage from '../SeatsPage'
import type { RoomPage, RoomViewport } from './types'

type RoomStageProps = {
	pages: RoomPage[]
	currentPageIndex: number
	menuImageMap: Map<string, string>
	viewport: RoomViewport
}

const RoomStage: FC<RoomStageProps> = ({
	pages,
	currentPageIndex,
	menuImageMap,
	viewport,
}) => (
	<div
		style={{
			position: 'relative',
			display: 'flex',
			alignItems: 'center',
			justifyContent: 'center',
			width: viewport.width,
			height: viewport.height,
		}}
	>
		{pages.map((page, index) => (
			<SeatsPage
				key={`${page.memberOnly ? 'member' : 'general'}-${page.firstSeatId}`}
				firstSeatId={page.firstSeatId}
				roomLayout={page.roomLayout}
				usedSeats={page.usedSeats}
				display={index === currentPageIndex}
				memberOnly={page.memberOnly}
				menuImageMap={menuImageMap}
				viewport={viewport}
			/>
		))}
	</div>
)

export default RoomStage
