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
		(itemAndImage) =>
			itemAndImage && (
				<div
					key={itemAndImage.item.code}
					css={[styles.listItem, styles.image]}
					style={{ width: '90px', height: '90px' }}
				>
					<Image
						src={itemAndImage.imageUrl}
						alt="menu item"
						fill
						style={{ objectFit: 'contain' }}
					/>
				</div>
			),
	)

	const nameList = [propsMemo.first, propsMemo.second].map(
		(itemAndImage) =>
			itemAndImage && (
				<div key={itemAndImage.item.code} css={[styles.listItem, styles.name]}>
					{itemAndImage.item.name}
				</div>
			),
	)

	const commandList = [propsMemo.firstNumber, propsMemo.secondNumber].map(
		(number) =>
			number && (
				<div key={`order-${number}`}>
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
