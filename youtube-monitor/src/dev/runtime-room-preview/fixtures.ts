import { Timestamp } from 'firebase/firestore'
import type { Seat } from '../../types/api'
import type { RuntimeRoomOverlayMap } from './overlay-map'

export type RuntimeRoomPreviewFixture = {
	overlayMap: RuntimeRoomOverlayMap
	usedSeats: Seat[]
}

const now = Date.now()

function createSeat(seatId: number, overrides: Partial<Seat> = {}): Seat {
	return {
		seat_id: seatId,
		user_id: `preview-user-${seatId.toString()}`,
		user_display_name: `プレビュー利用者${seatId.toString()}`,
		work_name: '実装とレビュー',
		break_work_name: '',
		entered_at: Timestamp.fromMillis(now - 45 * 60 * 1000),
		until: Timestamp.fromMillis(now + 45 * 60 * 1000),
		appearance: {
			color_code1: '#5BD27D',
			color_code2: '#008CFF',
			num_stars: seatId === 5 ? 5 : 0,
			color_gradient_enabled: seatId === 5,
		},
		menu_code: '',
		state: 'work',
		current_state_started_at: Timestamp.fromMillis(now - 45 * 60 * 1000),
		current_state_until: Timestamp.fromMillis(now + 45 * 60 * 1000),
		cumulative_work_sec: 45 * 60,
		daily_cumulative_work_sec: 45 * 60,
		user_profile_image_url: '',
		...overrides,
	}
}

const previewSeatStates: Seat[] = [
	createSeat(2, { user_display_name: '一般利用中', work_name: '設計レビュー' }),
	createSeat(3, {
		user_display_name: '長い作業名の確認者',
		work_name:
			'Runtime Room画像と実寸SeatBoxの互換性を細部まで確認する長い作業名',
	}),
	createSeat(4, {
		user_display_name: '休憩中の利用者',
		work_name: '実装作業',
		break_work_name: 'コーヒー休憩',
		state: 'break',
	}),
	createSeat(5, {
		user_display_name: 'プロフィール画像あり',
		work_name: 'メンバー表示確認',
		user_profile_image_url: '/images/sample_profile.svg',
	}),
]

export const passingOverlayMap: RuntimeRoomOverlayMap = {
	floor_image: '',
	font_size_ratio: 0.015,
	room_shape: { width: 1520, height: 1000 },
	seat_shape: { width: 140, height: 100 },
	partition_shapes: [],
	partitions: [],
	seats: [
		{ id: 1, x: 70, y: 100, rotate: 0 },
		{ id: 2, x: 360, y: 100, rotate: 0 },
		{ id: 3, x: 650, y: 100, rotate: 0 },
		{ id: 4, x: 940, y: 100, rotate: 0 },
		{ id: 5, x: 110, y: 630, rotate: 0 },
		{ id: 6, x: 400, y: 630, rotate: 0 },
		{ id: 7, x: 690, y: 630, rotate: 0 },
		{ id: 8, x: 980, y: 630, rotate: 0 },
	],
	runtime_preview: {
		id: 'passing-split-islands',
		name: '正常系 — Split Islands 8席',
		description:
			'一般席・メンバー席ともに、全SeatBoxがRoom内・Seat Zone内へ収まり、保護領域と主動線を避ける正常系。',
		runtime_profile: 'both',
		camera_profile: 'medium slightly elevated 3/4',
		protected_visual_zones: [
			{
				id: 'PVZ-hero-view',
				label: 'ヒーロー景観',
				shape: { type: 'rect', x: 1260, y: 60, width: 220, height: 790 },
			},
		],
		seat_zones: [
			{
				id: 'SZ-upper',
				label: '上段ワークステーション群',
				shape: { type: 'rect', x: 40, y: 60, width: 1190, height: 230 },
			},
			{
				id: 'SZ-lower',
				label: '下段ワークステーション群',
				shape: { type: 'rect', x: 40, y: 590, width: 1190, height: 230 },
			},
		],
		main_circulation: [
			{
				id: 'MC-center',
				label: '中央主動線',
				shape: { type: 'rect', x: 40, y: 350, width: 1440, height: 150 },
			},
		],
		seat_zone_by_seat_id: {
			1: 'SZ-upper',
			2: 'SZ-upper',
			3: 'SZ-upper',
			4: 'SZ-upper',
			5: 'SZ-lower',
			6: 'SZ-lower',
			7: 'SZ-lower',
			8: 'SZ-lower',
		},
	},
}

/**
 * Calo current相当の既知失敗を固定する回帰fixture。
 * SeatBoxを画像へ合わせて縮小・再配置せず、短いベイ、港景、横動線との
 * 既知の競合をvalidatorが検出できる状態で保持する。
 */
export const caloRegressionOverlayMap: RuntimeRoomOverlayMap = {
	floor_image: '',
	font_size_ratio: 0.015,
	room_shape: { width: 1520, height: 1000 },
	seat_shape: { width: 140, height: 100 },
	partition_shapes: [],
	partitions: [],
	seats: [
		{ id: 1, x: 70, y: 160, rotate: 0 },
		{ id: 2, x: 245, y: 150, rotate: 3 },
		{ id: 3, x: 420, y: 155, rotate: -3 },
		{ id: 4, x: 680, y: 220, rotate: 0 },
		{ id: 5, x: 805, y: 225, rotate: 2 },
		{ id: 6, x: 980, y: 220, rotate: -2 },
		{ id: 7, x: 1150, y: 230, rotate: 0 },
		{ id: 8, x: 160, y: 570, rotate: 0 },
		{ id: 9, x: 330, y: 600, rotate: -4 },
		{ id: 10, x: 560, y: 620, rotate: 3 },
	],
	runtime_preview: {
		id: 'calo-current-regression',
		name: 'Calo current相当 — 既知UI互換性問題',
		description:
			'10席と港湾Hubの骨格は維持し、短いベイのSeatBox衝突、港景の侵入、横動線の侵入を意図的に再現する回帰fixture。成功例ではない。',
		runtime_profile: 'both',
		camera_profile: 'Offset Spine / medium-high slightly elevated',
		protected_visual_zones: [
			{
				id: 'PVZ-harbor-view',
				label: '港景・水平線',
				shape: {
					type: 'polygon',
					points: [
						{ x: 1180, y: 40 },
						{ x: 1500, y: 40 },
						{ x: 1500, y: 420 },
						{ x: 1240, y: 390 },
					],
				},
			},
		],
		seat_zones: [
			{
				id: 'SZ-west-bay',
				label: '西側短ベイ',
				shape: { type: 'rect', x: 40, y: 110, width: 540, height: 230 },
			},
			{
				id: 'SZ-east-bay',
				label: '東側短ベイ',
				shape: { type: 'rect', x: 650, y: 180, width: 640, height: 230 },
			},
			{
				id: 'SZ-south-bay',
				label: '南側短ベイ',
				shape: { type: 'rect', x: 120, y: 550, width: 650, height: 230 },
			},
		],
		main_circulation: [
			{
				id: 'MC-harbor-spine',
				label: '港湾横動線',
				shape: { type: 'rect', x: 40, y: 440, width: 1440, height: 160 },
			},
		],
		seat_zone_by_seat_id: {
			1: 'SZ-west-bay',
			2: 'SZ-west-bay',
			3: 'SZ-west-bay',
			4: 'SZ-east-bay',
			5: 'SZ-east-bay',
			6: 'SZ-east-bay',
			7: 'SZ-east-bay',
			8: 'SZ-south-bay',
			9: 'SZ-south-bay',
			10: 'SZ-south-bay',
		},
	},
}

export const runtimeRoomPreviewFixtures: RuntimeRoomPreviewFixture[] = [
	{
		overlayMap: passingOverlayMap,
		usedSeats: previewSeatStates,
	},
	{
		overlayMap: caloRegressionOverlayMap,
		usedSeats: previewSeatStates,
	},
]
