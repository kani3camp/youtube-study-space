/** @jsxImportSource @emotion/react */
import { useEffect, useMemo, useRef, useState } from 'react'
import SeatsPage from '../components/SeatsPage'
import {
	Constants,
	type RoomConfigName,
	roomConfigNames,
} from '../lib/constants'
import {
	getRoomGalleryEntries,
	type RoomGalleryEntry,
} from '../rooms/room-gallery'
import * as styles from '../styles/RoomGallery.styles'
import {
	createRoomGallerySeats,
	type RoomGallerySeatState,
	roomGallerySeatStates,
} from './room-gallery-fixtures'

const menuImageMap = new Map<string, string>([
	['coffee', '/images/menu_default.svg'],
])

const profileOptions = ['General', 'Member'] as const

const getRoomShape = (entry: RoomGalleryEntry) => {
	const frameWidth = Constants.screenWidth - Constants.sideBarWidth
	const frameHeight = Constants.screenHeight - Constants.messageBarHeight
	const frameRatio = frameWidth / frameHeight
	const roomRatio =
		entry.layout.room_shape.width / entry.layout.room_shape.height

	return roomRatio >= frameRatio
		? { widthPx: frameWidth, heightPx: frameWidth / roomRatio }
		: { widthPx: frameHeight * roomRatio, heightPx: frameHeight }
}

type RoomPreviewProps = {
	entry: RoomGalleryEntry
	memberOnly: boolean
	seatState: RoomGallerySeatState
	detail?: boolean
}

const RoomPreview = ({
	entry,
	memberOnly,
	seatState,
	detail = false,
}: RoomPreviewProps) => {
	const shellRef = useRef<HTMLDivElement>(null)
	const [shellWidth, setShellWidth] = useState(0)
	const roomShape = useMemo(() => getRoomShape(entry), [entry])
	const usedSeats = useMemo(
		() => createRoomGallerySeats(seatState, entry.seatCount),
		[entry.seatCount, seatState],
	)

	useEffect(() => {
		const shell = shellRef.current
		if (!shell) {
			return
		}

		const updateWidth = () => {
			setShellWidth(shell.getBoundingClientRect().width)
		}
		updateWidth()

		if (typeof ResizeObserver === 'undefined') {
			return
		}
		const observer = new ResizeObserver(updateWidth)
		observer.observe(shell)
		return () => observer.disconnect()
	}, [])

	const scale = shellWidth > 0 ? shellWidth / roomShape.widthPx : 1
	return (
		<div
			ref={shellRef}
			css={[styles.previewShell, detail && styles.detailPreview]}
			style={{
				aspectRatio: `${entry.layout.room_shape.width} / ${entry.layout.room_shape.height}`,
			}}
		>
			{!entry.layout.floor_image && (
				<div css={styles.noImageLabel}>No image</div>
			)}
			<div
				css={styles.previewCanvas}
				style={{
					width: roomShape.widthPx,
					height: roomShape.heightPx,
					transform: `scale(${scale})`,
				}}
			>
				<SeatsPage
					roomLayout={entry.layout}
					usedSeats={usedSeats}
					firstSeatId={1}
					display={true}
					memberOnly={memberOnly}
					menuImageMap={menuImageMap}
				/>
			</div>
		</div>
	)
}

const ToggleButton = ({
	active,
	children,
	onClick,
}: {
	active: boolean
	children: string
	onClick: () => void
}) => (
	<button
		type="button"
		css={[styles.segment, active && styles.activeSegment]}
		aria-pressed={active}
		onClick={onClick}
	>
		{children}
	</button>
)

const RoomCard = ({
	entry,
	memberOnly,
	seatState,
	selected,
	onSelect,
}: RoomPreviewProps & { selected: boolean; onSelect: () => void }) => (
	<button
		type="button"
		css={[styles.roomCard, selected && styles.selectedRoomCard]}
		aria-pressed={selected}
		aria-label={`${entry.displayName} (${entry.id})`}
		onClick={onSelect}
	>
		<RoomPreview entry={entry} memberOnly={memberOnly} seatState={seatState} />
		<div css={styles.cardBody}>
			<div css={styles.cardTitleRow}>
				<span css={styles.cardTitle}>{entry.displayName}</span>
				<span css={styles.roomId}>{entry.id}</span>
			</div>
			<div css={styles.badgeRow}>
				<span
					css={[
						styles.badge,
						entry.enabled ? styles.enabledBadge : styles.disabledBadge,
					]}
				>
					{entry.enabled ? 'Enabled' : 'Disabled'}
				</span>
			</div>
			<div css={styles.cardMeta}>
				{entry.categories[0] && <span>{entry.categories[0]}</span>}
				<span>{entry.seatCount} seats</span>
				{!entry.layout.floor_image && <span>No image</span>}
			</div>
		</div>
	</button>
)

const RoomGallery = () => {
	const [configName, setConfigName] = useState<RoomConfigName>('PROD')
	const [memberOnly, setMemberOnly] = useState(false)
	const [seatState, setSeatState] =
		useState<RoomGallerySeatState>('Representative')
	const entries = useMemo(() => getRoomGalleryEntries(configName), [configName])
	const [selectedId, setSelectedId] = useState(entries[0]?.id)

	useEffect(() => {
		setSelectedId(entries.find((entry) => entry.enabled)?.id ?? entries[0]?.id)
	}, [entries])

	const selectedEntry =
		entries.find((entry) => entry.id === selectedId) ?? entries[0]
	const enabledEntries = entries.filter((entry) => entry.enabled)
	const otherEntries = entries.filter((entry) => !entry.enabled)

	if (!selectedEntry) {
		return null
	}

	return (
		<div css={styles.page}>
			<header css={styles.header}>
				<h1 css={styles.title}>Room Gallery</h1>
			</header>

			<div css={styles.toolbar}>
				<div css={styles.controlGroup}>
					<span css={styles.controlLabel}>Room config</span>
					<div css={styles.segmentedControl}>
						{roomConfigNames.map((name) => (
							<ToggleButton
								key={name}
								active={configName === name}
								onClick={() => setConfigName(name)}
							>
								{name}
							</ToggleButton>
						))}
					</div>
				</div>
				<div css={styles.controlGroup}>
					<span css={styles.controlLabel}>Seat profile</span>
					<div css={styles.segmentedControl}>
						{profileOptions.map((profile) => (
							<ToggleButton
								key={profile}
								active={memberOnly === (profile === 'Member')}
								onClick={() => setMemberOnly(profile === 'Member')}
							>
								{profile}
							</ToggleButton>
						))}
					</div>
				</div>
				<div css={styles.controlGroup}>
					<span css={styles.controlLabel}>Seat state</span>
					<div css={styles.segmentedControl}>
						{roomGallerySeatStates.map((state) => (
							<ToggleButton
								key={state}
								active={seatState === state}
								onClick={() => setSeatState(state)}
							>
								{state}
							</ToggleButton>
						))}
					</div>
				</div>
			</div>

			<div css={styles.workspace}>
				<main css={styles.list}>
					<RoomSection
						title="Currently Enabled"
						entries={enabledEntries}
						memberOnly={memberOnly}
						seatState={seatState}
						selectedId={selectedId}
						onSelect={setSelectedId}
					/>
					<RoomSection
						title="Other Rooms"
						entries={otherEntries}
						memberOnly={memberOnly}
						seatState={seatState}
						selectedId={selectedId}
						onSelect={setSelectedId}
					/>
				</main>

				<aside css={styles.detailPanel} aria-label="Detail Preview">
					<div css={styles.detailHeading}>
						<div>
							<h2 css={styles.detailTitle}>{selectedEntry.displayName}</h2>
							<p css={styles.detailDescription}>{selectedEntry.id}</p>
						</div>
						<span
							css={[
								styles.badge,
								selectedEntry.enabled
									? styles.enabledBadge
									: styles.disabledBadge,
							]}
						>
							{selectedEntry.enabled ? 'Enabled' : 'Disabled'}
						</span>
					</div>
					<RoomPreview
						entry={selectedEntry}
						memberOnly={memberOnly}
						seatState={seatState}
						detail={true}
					/>
					<dl css={styles.detailMeta}>
						<DetailMeta
							label="Seats"
							value={selectedEntry.seatCount.toString()}
						/>
						<DetailMeta
							label="Floor image"
							value={
								selectedEntry.layout.floor_image ? 'Available' : 'No image'
							}
						/>
						<DetailMeta
							label="Profile"
							value={memberOnly ? 'Member' : 'General'}
						/>
						<DetailMeta label="State" value={seatState} />
					</dl>
				</aside>
			</div>
		</div>
	)
}

const RoomSection = ({
	title,
	entries,
	memberOnly,
	seatState,
	selectedId,
	onSelect,
}: {
	title: string
	entries: RoomGalleryEntry[]
	memberOnly: boolean
	seatState: RoomGallerySeatState
	selectedId: RoomGalleryEntry['id'] | undefined
	onSelect: (id: RoomGalleryEntry['id']) => void
}) => (
	<section css={styles.section}>
		<div css={styles.sectionHeader}>
			<h2 css={styles.sectionTitle}>{title}</h2>
			<span css={styles.sectionCount}>{entries.length} rooms</span>
		</div>
		<div css={styles.cards}>
			{entries.map((entry) => (
				<RoomCard
					key={entry.id}
					entry={entry}
					memberOnly={memberOnly}
					seatState={seatState}
					selected={entry.id === selectedId}
					onSelect={() => onSelect(entry.id)}
				/>
			))}
		</div>
	</section>
)

const DetailMeta = ({ label, value }: { label: string; value: string }) => (
	<div css={styles.detailMetaItem}>
		<dt css={styles.detailMetaLabel}>{label}</dt>
		<dd css={styles.detailMetaValue}>{value}</dd>
	</div>
)

export default RoomGallery
