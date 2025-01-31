import Image from 'next/image'
import { type FC, useMemo } from 'react'
import * as styles from '../styles/Menu.styles'
import type { MenuItemAndImage } from './MenuDisplay'

export type MenuBoxProps = {
	first: MenuItemAndImage
	firstNumber: number
	second: MenuItemAndImage | null
	secondNumber: number | null
	display: boolean
}

const MenuBox: FC<MenuBoxProps> = (props: MenuBoxProps) => {
	const propsMemo = useMemo(() => props, [props])

	const imageList = [propsMemo.first, propsMemo.second].map(
		(itemAndImage, i) =>
			itemAndImage && (
				<Image
					key={i}
					src={itemAndImage.imageUrl}
					alt="menu item"
					width={90}
					height={90}
					css={[styles.listItem, styles.image]}
				/>
			),
	)

	const nameList = [propsMemo.first, propsMemo.second].map(
		(itemAndImage, i) =>
			itemAndImage && (
				<div key={i} css={[styles.listItem, styles.name]}>
					{itemAndImage.item.name}
				</div>
			),
	)

	const commandList = [propsMemo.firstNumber, propsMemo.secondNumber].map(
		(number, i) =>
			number && (
				<div key={i}>
					<span css={[styles.listItem, styles.commandCode]}>
						!order {number}
					</span>
				</div>
			),
	)

	return (
		<div
			style={{
				display: propsMemo.display ? 'block' : 'none',
				position: 'absolute',
				width: '100%',
			}}
		>
			<div css={styles.list}>{imageList}</div>
			<div css={styles.list}>{nameList}</div>
			<div css={styles.list}>{commandList}</div>{' '}
		</div>
	)
}

export default MenuBox
