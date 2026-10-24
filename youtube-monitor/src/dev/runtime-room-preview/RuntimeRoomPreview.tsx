import { type FC, useMemo, useState } from 'react'
import SeatsPage from '../../components/SeatsPage'
import { Constants } from '../../lib/constants'
import type { RuntimeRoomPreviewFixture } from './fixtures'
import { runtimeRoomPreviewFixtures } from './fixtures'
import type { OverlayZone, RuntimeProfile } from './overlay-map'
import { roomLayoutForProfile, runtimeProfiles } from './overlay-map'
import * as styles from './RuntimeRoomPreview.styles'
import { validateRuntimeRoom } from './validator'

type OverlayVisibility = {
	seatZones: boolean
	protectedVisualZones: boolean
	mainCirculation: boolean
	seatAnchors: boolean
	boundingBoxes: boolean
	validationState: boolean
}

const initialVisibility: OverlayVisibility = {
	seatZones: true,
	protectedVisualZones: true,
	mainCirculation: true,
	seatAnchors: true,
	boundingBoxes: true,
	validationState: true,
}

const roomWidth = Constants.screenWidth - Constants.sideBarWidth
const roomHeight = Constants.screenHeight - Constants.messageBarHeight
const messageWidth = roomWidth - Constants.tickerWidth

type Props = {
	fixtures?: RuntimeRoomPreviewFixture[]
	initialFixtureId?: string
}

function zonePoints(zone: OverlayZone): string {
	if (zone.shape.type === 'polygon') {
		return zone.shape.points.map((point) => `${point.x},${point.y}`).join(' ')
	}
	const { x, y, width, height } = zone.shape
	return [
		`${x},${y}`,
		`${x + width},${y}`,
		`${x + width},${y + height}`,
		`${x},${y + height}`,
	].join(' ')
}

const RuntimeRoomPreview: FC<Props> = ({
	fixtures = runtimeRoomPreviewFixtures,
	initialFixtureId,
}) => {
	const availableFixtures =
		fixtures.length > 0 ? fixtures : runtimeRoomPreviewFixtures
	const firstFixture =
		availableFixtures.find(
			(candidate) =>
				candidate.overlayMap.runtime_preview.id === initialFixtureId,
		) ?? availableFixtures[0]
	const [fixtureId, setFixtureId] = useState(
		firstFixture.overlayMap.runtime_preview.id,
	)
	const [profile, setProfile] = useState<RuntimeProfile>('general')
	const [scale, setScale] = useState(0.45)
	const [visibility, setVisibility] =
		useState<OverlayVisibility>(initialVisibility)
	const fixture =
		availableFixtures.find(
			(candidate) => candidate.overlayMap.runtime_preview.id === fixtureId,
		) ?? firstFixture
	const { overlayMap, usedSeats } = fixture
	const roomLayout = useMemo(
		() => roomLayoutForProfile(overlayMap, profile),
		[overlayMap, profile],
	)
	const validation = useMemo(
		() => validateRuntimeRoom(overlayMap, profile),
		[overlayMap, profile],
	)
	const invalidSeatIds = useMemo(() => {
		const ids = new Set<number>(validation.overflows)
		for (const collision of validation.seatCollisions) {
			ids.add(collision.seatIds[0])
			ids.add(collision.seatIds[1])
		}
		for (const intrusion of [
			...validation.protectedVisualZoneIntrusions,
			...validation.mainCirculationIntrusions,
			...validation.seatZoneMisses,
		]) {
			ids.add(intrusion.seatId)
		}
		return ids
	}, [validation])

	const updateVisibility = (key: keyof OverlayVisibility) => {
		setVisibility((current) => ({ ...current, [key]: !current[key] }))
	}
	const statusText = (seatIds: number[]) =>
		seatIds.length === 0 ? 'なし' : seatIds.join(', ')
	const intrusionText = (items: { seatId: number; zoneId: string }[]) =>
		items.length === 0
			? 'なし'
			: items.map((item) => `${item.seatId}→${item.zoneId}`).join(', ')

	return (
		<div css={styles.preview}>
			<div css={styles.controls}>
				<label css={styles.field}>
					Fixture
					<select
						value={fixtureId}
						onChange={(event) => setFixtureId(event.target.value)}
					>
						{availableFixtures.map((candidate) => (
							<option
								key={candidate.overlayMap.runtime_preview.id}
								value={candidate.overlayMap.runtime_preview.id}
							>
								{candidate.overlayMap.runtime_preview.name}
							</option>
						))}
					</select>
				</label>
				<div css={styles.profileButtons}>
					{(['general', 'member'] as const).map((candidate) => (
						<button
							type="button"
							key={candidate}
							css={[
								styles.profileButton,
								candidate === profile && styles.selectedProfileButton,
							]}
							onClick={() => setProfile(candidate)}
						>
							{runtimeProfiles[candidate].label}
						</button>
					))}
				</div>
				<label css={styles.field}>
					表示倍率
					<input
						type="range"
						min="0.25"
						max="0.75"
						step="0.05"
						value={scale}
						onChange={(event) => setScale(Number(event.target.value))}
					/>
					{Math.round(scale * 100)}%
				</label>
				{(
					[
						['seatZones', 'Seat Zone'],
						['protectedVisualZones', 'Protected Visual Zone'],
						['mainCirculation', 'main circulation'],
						['seatAnchors', 'Seat Anchor'],
						['boundingBoxes', 'SeatBox外接矩形'],
						['validationState', 'collision / overflow状態'],
					] as const
				).map(([key, label]) => (
					<label css={styles.field} key={key}>
						<input
							type="checkbox"
							checked={visibility[key]}
							onChange={() => updateVisibility(key)}
						/>
						{label}
					</label>
				))}
				<p css={styles.fixtureDescription}>
					{overlayMap.runtime_preview.description} Camera:{' '}
					{overlayMap.runtime_preview.camera_profile}
				</p>
			</div>

			<div
				css={styles.scaledFrame}
				style={{
					width: Constants.screenWidth * scale,
					height: Constants.screenHeight * scale,
				}}
			>
				<div
					css={styles.fullFrame}
					style={{
						width: Constants.screenWidth,
						height: Constants.screenHeight,
						transform: `scale(${scale})`,
					}}
				>
					<div
						css={styles.roomRegion}
						style={{ width: roomWidth, height: roomHeight }}
					>
						{!roomLayout.floor_image && (
							<div css={styles.cleanImagePlaceholder} />
						)}
						<SeatsPage
							roomLayout={roomLayout}
							usedSeats={usedSeats}
							firstSeatId={1}
							display={true}
							memberOnly={runtimeProfiles[profile].memberOnly}
							menuImageMap={new Map<string, string>()}
						/>
						<svg
							css={styles.overlay}
							viewBox={`0 0 ${roomLayout.room_shape.width} ${roomLayout.room_shape.height}`}
							aria-label="Runtime Room development overlay"
						>
							{visibility.seatZones &&
								overlayMap.runtime_preview.seat_zones.map((zone) => (
									<polygon
										key={zone.id}
										points={zonePoints(zone)}
										fill="rgba(43, 135, 255, 0.13)"
										stroke="#1870d5"
										strokeWidth="3"
										strokeDasharray="12 8"
									/>
								))}
							{visibility.protectedVisualZones &&
								overlayMap.runtime_preview.protected_visual_zones.map(
									(zone) => (
										<polygon
											key={zone.id}
											points={zonePoints(zone)}
											fill="rgba(255, 54, 109, 0.16)"
											stroke="#d91f55"
											strokeWidth="4"
										/>
									),
								)}
							{visibility.mainCirculation &&
								overlayMap.runtime_preview.main_circulation.map((zone) => (
									<polygon
										key={zone.id}
										points={zonePoints(zone)}
										fill="rgba(255, 182, 33, 0.18)"
										stroke="#c27400"
										strokeWidth="4"
										strokeDasharray="18 10"
									/>
								))}
							{visibility.boundingBoxes &&
								validation.seatBounds.map((bounds) => (
									<rect
										key={bounds.seatId}
										x={bounds.left}
										y={bounds.top}
										width={bounds.right - bounds.left}
										height={bounds.bottom - bounds.top}
										fill="none"
										stroke={
											visibility.validationState &&
											invalidSeatIds.has(bounds.seatId)
												? '#e00000'
												: '#145c2e'
										}
										strokeWidth="5"
									/>
								))}
							{visibility.seatAnchors &&
								overlayMap.seats.map((seat) => (
									<g key={seat.id}>
										<circle cx={seat.x} cy={seat.y} r="10" fill="#24105f" />
										<path
											d={`M ${seat.x - 18} ${seat.y} H ${seat.x + 18} M ${seat.x} ${seat.y - 18} V ${seat.y + 18}`}
											stroke="white"
											strokeWidth="4"
										/>
									</g>
								))}
						</svg>
					</div>
					<div
						css={[styles.frameRegion, styles.sidebarRegion]}
						style={{
							width: Constants.sideBarWidth,
							height: Constants.screenHeight,
						}}
					>
						Sidebar {Constants.sideBarWidth} × {Constants.screenHeight}
					</div>
					<div
						css={[styles.frameRegion, styles.bottomRegion]}
						style={{
							left: 0,
							width: messageWidth,
							height: Constants.messageBarHeight,
						}}
					>
						Message {messageWidth} × {Constants.messageBarHeight}
					</div>
					<div
						css={[styles.frameRegion, styles.bottomRegion]}
						style={{
							left: messageWidth,
							width: Constants.tickerWidth,
							height: Constants.messageBarHeight,
						}}
					>
						Ticker {Constants.tickerWidth} × {Constants.messageBarHeight}
					</div>
				</div>
			</div>

			{visibility.validationState && (
				<div css={styles.statusPanel}>
					<p
						css={[
							styles.statusSummary,
							validation.isValid ? styles.valid : styles.invalid,
						]}
					>
						{validation.isValid ? 'PASS' : 'FAIL'} —{' '}
						{runtimeProfiles[profile].label}
					</p>
					<p css={styles.statusItem}>
						衝突:{' '}
						{validation.seatCollisions.length === 0
							? 'なし'
							: validation.seatCollisions
									.map((item) => item.seatIds.join('↔'))
									.join(', ')}
					</p>
					<p css={styles.statusItem}>
						Room外: {statusText(validation.overflows)}
					</p>
					<p css={styles.statusItem}>
						Protected侵入:{' '}
						{intrusionText(validation.protectedVisualZoneIntrusions)}
					</p>
					<p css={styles.statusItem}>
						主動線侵入: {intrusionText(validation.mainCirculationIntrusions)}
					</p>
					<p css={styles.statusItem}>
						Seat Zone外: {intrusionText(validation.seatZoneMisses)}
					</p>
				</div>
			)}
		</div>
	)
}

export default RuntimeRoomPreview
