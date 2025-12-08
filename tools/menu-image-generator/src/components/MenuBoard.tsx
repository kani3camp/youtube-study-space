import type React from 'react'
import type { MenuItemWithNumber } from '../types'

/**
 * HSLからHEXに変換
 */
function hslToHex(h: number, s: number, l: number): string {
	// hue値を[0, 360)の範囲に正規化
	const normalizedH = ((h % 360) + 360) % 360

	const sNorm = s / 100
	const lNorm = l / 100

	const c = (1 - Math.abs(2 * lNorm - 1)) * sNorm
	const x = c * (1 - Math.abs(((normalizedH / 60) % 2) - 1))
	const m = lNorm - c / 2

	let r = 0
	let g = 0
	let b = 0

	if (normalizedH >= 0 && normalizedH < 60) {
		r = c
		g = x
		b = 0
	} else if (normalizedH >= 60 && normalizedH < 120) {
		r = x
		g = c
		b = 0
	} else if (normalizedH >= 120 && normalizedH < 180) {
		r = 0
		g = c
		b = x
	} else if (normalizedH >= 180 && normalizedH < 240) {
		r = 0
		g = x
		b = c
	} else if (normalizedH >= 240 && normalizedH < 300) {
		r = x
		g = 0
		b = c
	} else {
		r = c
		g = 0
		b = x
	}

	const toHex = (n: number) => {
		const hex = Math.round((n + m) * 255).toString(16)
		return hex.length === 1 ? `0${hex}` : hex
	}

	return `#${toHex(r)}${toHex(g)}${toHex(b)}`
}

/**
 * ランダムなカラフルグラデーションを生成
 * HSL色空間を使用して調和のとれた色の組み合わせを作成
 */
function generateRandomGradient(): string {
	// ベースの色相をランダムに選択（0-360度）
	const baseHue = Math.floor(Math.random() * 360)

	// グラデーションのタイプをランダムに選択
	const gradientType = Math.floor(Math.random() * 5)

	// より鮮やかなパステルカラー用の彩度と明度の範囲
	const saturation = 45 + Math.random() * 35 // 45-80%
	const lightness = 85 + Math.random() * 10 // 85-95%

	let colors: string[]

	switch (gradientType) {
		case 0:
			// 虹色グラデーション（広い色相範囲）
			colors = [
				hslToHex(baseHue, saturation, lightness),
				hslToHex((baseHue + 60) % 360, saturation, lightness - 3),
				hslToHex((baseHue + 120) % 360, saturation - 10, lightness),
				hslToHex((baseHue + 180) % 360, saturation, lightness - 3),
				hslToHex((baseHue + 240) % 360, saturation - 5, lightness),
			]
			break
		case 1:
			// サンセット風（暖色系）
			colors = [
				hslToHex(0 + Math.random() * 30, saturation + 10, lightness), // 赤〜オレンジ
				hslToHex(30 + Math.random() * 20, saturation, lightness - 2),
				hslToHex(45 + Math.random() * 15, saturation - 5, lightness),
				hslToHex(280 + Math.random() * 40, saturation - 10, lightness - 2), // ピンク〜紫
				hslToHex(320 + Math.random() * 30, saturation, lightness),
			]
			break
		case 2:
			// オーシャン風（寒色系）
			colors = [
				hslToHex(180 + Math.random() * 40, saturation + 10, lightness), // シアン
				hslToHex(200 + Math.random() * 30, saturation, lightness - 2),
				hslToHex(220 + Math.random() * 20, saturation - 5, lightness), // 青
				hslToHex(160 + Math.random() * 30, saturation, lightness - 3), // ターコイズ
				hslToHex(140 + Math.random() * 30, saturation - 10, lightness), // 緑寄り
			]
			break
		case 3:
			// ネオン風（高彩度）
			colors = [
				hslToHex(baseHue, saturation + 15, lightness - 5),
				hslToHex((baseHue + 90) % 360, saturation + 10, lightness - 3),
				hslToHex((baseHue + 180) % 360, saturation + 15, lightness - 5),
				hslToHex((baseHue + 270) % 360, saturation + 10, lightness - 3),
				hslToHex(baseHue, saturation + 5, lightness),
			]
			break
		default:
			// パステルレインボー
			colors = [
				hslToHex(baseHue, saturation - 10, lightness + 3),
				hslToHex((baseHue + 72) % 360, saturation, lightness),
				hslToHex((baseHue + 144) % 360, saturation - 5, lightness + 2),
				hslToHex((baseHue + 216) % 360, saturation, lightness),
				hslToHex((baseHue + 288) % 360, saturation - 10, lightness + 3),
			]
			break
	}

	// グラデーションの角度もランダムに
	const angle = 120 + Math.floor(Math.random() * 30) // 120-150度

	return `linear-gradient(${angle}deg, ${colors[0]} 0%, ${colors[1]} 25%, ${colors[2]} 50%, ${colors[3]} 75%, ${colors[4]} 100%)`
}

// デザイン定数
const DESIGN = {
	imageSize: 2048,
	fontFamily: "'M PLUS Rounded 1c', 'Noto Sans JP', sans-serif",
	// フォントサイズ
	titleFontSize: 120,
	itemNameFontSize: 62,
	commandFontSize: 52,
	noticeFontSize: 46,
	// レイアウト
	padding: 50,
	gap: 28,
	gridBottomPadding: 80,
	// タイトル
	titleEmojiSize: 100,
	titleGap: 32,
	titlePadding: 25,
	// 日付表示
	dateTop: 20,
	dateRight: 24,
	dateOpacity: 0.4,
	// フッター
	footerBottom: 30,
	footerEmojiSize: 32,
	// カラーパレット
	titleColor: '#2D6B8A',
	itemNameColor: '#3D4852',
	commandTextColor: '#2D6B8A',
	commandBgColor: 'rgba(255, 255, 255, 0.8)',
	commandBorderColor: '#2D6B8A',
	noticeColor: '#6B8A9A',
	// カード装飾
	cardBgColor: 'rgba(255, 255, 255, 0.45)',
	cardBorderRadius: 24,
	cardShadow: '0 4px 20px rgba(0, 0, 0, 0.08)',
} as const

// レイアウト設定（アイテム数に応じた調整用）
const LAYOUT_CONFIG = {
	thresholds: [4, 8] as const,
	scales: [1.0, 0.9, 0.8] as const,
	cardPadding: { normal: 20, compact: 14 },
	cardGap: { normal: 12, compact: 10 },
} as const

/**
 * アイテム数に応じた最適な列数を計算する
 * 1-4個→2列, 5-8個→3列, 9個以上→4列（最大3行x4列）
 */
function calculateColumns(itemCount: number): number {
	if (itemCount <= LAYOUT_CONFIG.thresholds[0]) return 2
	if (itemCount <= LAYOUT_CONFIG.thresholds[1]) return 3
	return 4
}

/**
 * アイテム数に応じたレイアウト設定を計算する
 */
function calculateLayout(itemCount: number) {
	const [t1, t2] = LAYOUT_CONFIG.thresholds
	const [s1, s2, s3] = LAYOUT_CONFIG.scales
	const scale = itemCount <= t1 ? s1 : itemCount <= t2 ? s2 : s3
	const isCompact = itemCount > t2
	return {
		scale,
		cardPadding: isCompact
			? LAYOUT_CONFIG.cardPadding.compact
			: LAYOUT_CONFIG.cardPadding.normal,
		cardGap: isCompact
			? LAYOUT_CONFIG.cardGap.compact
			: LAYOUT_CONFIG.cardGap.normal,
	}
}

/**
 * 日付をyyyy-MM-dd形式でフォーマットする
 */
function formatDate(date: Date): string {
	const y = date.getFullYear()
	const m = String(date.getMonth() + 1).padStart(2, '0')
	const d = String(date.getDate()).padStart(2, '0')
	return `${y}-${m}-${d}`
}

type MenuBoardProps = {
	items: MenuItemWithNumber[]
	pageNumber: number
	totalPages: number
}

/**
 * メニュー表のReactコンポーネント
 */
export const MenuBoard: React.FC<MenuBoardProps> = ({
	items,
	pageNumber,
	totalPages,
}) => {
	// レイアウト計算
	const columns = calculateColumns(items.length)
	const layout = calculateLayout(items.length)
	const { scale, cardPadding, cardGap } = layout

	// グリッドのサイズ計算
	const contentWidth = DESIGN.imageSize - DESIGN.padding * 2
	const cellWidth = contentWidth / columns
	const imageSize = cellWidth * 0.55

	// 動的なフォントサイズ・ギャップ
	const itemNameFontSize = Math.floor(DESIGN.itemNameFontSize * scale)
	const commandFontSize = Math.floor(DESIGN.commandFontSize * scale)
	const gap = Math.floor(DESIGN.gap * scale)

	// 日付文字列
	const dateString = formatDate(new Date())

	return (
		<div
			style={{
				width: DESIGN.imageSize,
				height: DESIGN.imageSize,
				background: generateRandomGradient(),
				padding: DESIGN.padding,
				boxSizing: 'border-box',
				fontFamily: DESIGN.fontFamily,
				display: 'flex',
				flexDirection: 'column',
				overflow: 'hidden',
				position: 'relative',
			}}
		>
			{/* 右上の日付表示 */}
			<div
				style={{
					position: 'absolute',
					top: DESIGN.dateTop,
					right: DESIGN.dateRight,
					fontSize: Math.floor(DESIGN.noticeFontSize * scale),
					color: `rgba(0, 0, 0, ${DESIGN.dateOpacity})`,
					fontWeight: 500,
				}}
			>
				{dateString}
			</div>

			{/* タイトル */}
			<div
				style={{
					display: 'flex',
					alignItems: 'center',
					justifyContent: 'center',
					gap: Math.floor(DESIGN.titleGap * scale),
					padding: `${Math.floor(DESIGN.titlePadding * scale)}px 0`,
					flexShrink: 0,
				}}
			>
				<span style={{ fontSize: Math.floor(DESIGN.titleEmojiSize * scale) }}>
					🍽️
				</span>
				<div
					style={{
						fontSize: Math.floor(DESIGN.titleFontSize * scale),
						fontWeight: 700,
						color: DESIGN.titleColor,
						textAlign: 'center',
					}}
				>
					メニュー
					{totalPages > 1 && (
						<span
							style={{
								fontSize: Math.floor(DESIGN.titleFontSize * scale * 0.5),
								marginLeft: 16,
								color: DESIGN.noticeColor,
							}}
						>
							({pageNumber}/{totalPages})
						</span>
					)}
				</div>
				<span style={{ fontSize: Math.floor(DESIGN.titleEmojiSize * scale) }}>
					🍽️
				</span>
			</div>

			{/* グリッド - 中央配置 */}
			<div
				style={{
					flex: 1,
					display: 'grid',
					gridTemplateColumns: `repeat(${columns}, 1fr)`,
					gridAutoRows: '1fr',
					placeContent: 'center',
					gap: gap,
					overflow: 'hidden',
					paddingBottom: DESIGN.gridBottomPadding,
				}}
			>
				{items.map((item) => (
					<div
						key={item.code}
						style={{
							display: 'flex',
							flexDirection: 'column',
							alignItems: 'center',
							justifyContent: 'center',
							gap: cardGap,
							padding: cardPadding,
							backgroundColor: DESIGN.cardBgColor,
							borderRadius: Math.floor(DESIGN.cardBorderRadius * scale),
							boxShadow: DESIGN.cardShadow,
							overflow: 'hidden',
						}}
					>
						{/* メニュー画像 */}
						<img
							src={item.image}
							alt={item.name}
							style={{
								width: imageSize,
								height: imageSize,
								objectFit: 'contain',
								filter: 'drop-shadow(0 4px 8px rgba(0, 0, 0, 0.1))',
								flexShrink: 0,
							}}
						/>

						{/* メニュー名（2行分の高さを確保） */}
						<div
							style={{
								fontSize: itemNameFontSize,
								fontWeight: 700,
								color: DESIGN.itemNameColor,
								textAlign: 'center',
								lineHeight: 1.2,
								maxWidth: '100%',
								height: `${itemNameFontSize * 1.2 * 2}px`,
								display: 'flex',
								alignItems: 'center',
								justifyContent: 'center',
							}}
						>
							{item.name}
						</div>

						{/* 注文コマンド */}
						<div
							style={{
								fontSize: commandFontSize,
								color: DESIGN.commandTextColor,
								textAlign: 'center',
								fontWeight: 700,
								backgroundColor: DESIGN.commandBgColor,
								border: `${Math.floor(3 * scale)}px solid ${DESIGN.commandBorderColor}`,
								padding: `${Math.floor(8 * scale)}px ${Math.floor(20 * scale)}px`,
								borderRadius: Math.floor(12 * scale),
								flexShrink: 0,
							}}
						>
							!order&nbsp;&nbsp;{item.number}
						</div>
					</div>
				))}
			</div>

			{/* フッター注意書き（絶対配置で下に固定） */}
			<div
				style={{
					position: 'absolute',
					bottom: DESIGN.footerBottom,
					left: 0,
					right: 0,
					fontSize: Math.floor(DESIGN.noticeFontSize * scale),
					color: DESIGN.noticeColor,
					textAlign: 'center',
					display: 'flex',
					alignItems: 'center',
					justifyContent: 'center',
					gap: Math.floor(LAYOUT_CONFIG.cardGap.normal * scale),
				}}
			>
				<span style={{ fontSize: Math.floor(DESIGN.footerEmojiSize * scale) }}>
					💡
				</span>
				※これは架空の注文機能であり、料金の請求は発生しません。
			</div>
		</div>
	)
}

/**
 * メニュー表のHTMLを生成する
 */
export function renderMenuBoardToHtml(
	items: MenuItemWithNumber[],
	pageNumber: number,
	totalPages: number,
): string {
	const ReactDOMServer = require('react-dom/server')

	const html = ReactDOMServer.renderToStaticMarkup(
		<MenuBoard items={items} pageNumber={pageNumber} totalPages={totalPages} />,
	)

	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="preconnect" href="https://fonts.googleapis.com">
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
	<link href="https://fonts.googleapis.com/css2?family=M+PLUS+Rounded+1c:wght@400;700;800;900&family=Noto+Sans+JP:wght@400;700;900&display=swap" rel="stylesheet">
	<style>
		* {
			margin: 0;
			padding: 0;
			box-sizing: border-box;
		}
		body {
			margin: 0;
			padding: 0;
			font-family: 'M PLUS Rounded 1c', 'Noto Sans JP', sans-serif;
		}
	</style>
</head>
<body>
	${html}
</body>
</html>`
}
