import { useTranslation } from 'next-i18next'
import { type FC, useEffect, useState, useCallback } from 'react'
import { useInterval } from '../lib/common'
import * as styles from '../styles/Menu.styles'
import { componentBackground, componentStyle } from '../styles/common.style'
import type { Menu } from '../types/api'
import MenuBox, { type MenuBoxProps } from './MenuBox'

const DEFAULT_MENU_IMAGE = '/images/menu_default.svg'

export type MenuItemAndImage = {
	item: Menu
	imageUrl: string
}

type MenuDisplayProps = {
	menuItems: Menu[]
}

const MenuDisplay: FC<MenuDisplayProps> = ({ menuItems }) => {
	const PAGING_INTERVAL_SEC = 5

	const { t } = useTranslation()

	const [menuBoxList, setMenuBoxList] = useState<MenuBoxProps[]>([])
	const [pageIndex, setPageIndex] = useState<number>(0)

	const updateMenuItems = useCallback(() => {
		const menuItemAndImages: MenuItemAndImage[] = menuItems.map(
			(item: Menu) => {
				const imageUrl = item.image || DEFAULT_MENU_IMAGE
				return { item, imageUrl }
			},
		)
		const newMenuBoxList: MenuBoxProps[] = []
		for (let i = 0; i < menuItemAndImages.length; i += 2) {
			const first = menuItemAndImages[i]
			const second = menuItemAndImages[i + 1] ? menuItemAndImages[i + 1] : null
			const firstNumber = i + 1
			const secondNumber = second ? i + 2 : null
			const display = i === 0
			newMenuBoxList.push({
				first,
				firstNumber,
				second,
				secondNumber,
				display,
			})
		}
		setMenuBoxList(newMenuBoxList)
	}, [menuItems])

	useEffect(() => {
		updateMenuItems()
	}, [updateMenuItems])

	useInterval(() => {
		refreshPageIndex()
	}, PAGING_INTERVAL_SEC * 1000)

	useEffect(() => {
		console.log('[currentMenuPageIndex]:', pageIndex)
		changePage(pageIndex)
	}, [pageIndex])

	const refreshPageIndex = () => {
		if (menuItems.length > 0) {
			const newPageIndex =
				(pageIndex + 1) % Math.ceil(menuItems.length / 2)
			setPageIndex(newPageIndex)
		}
	}

	const changePage = (pageIndex: number) => {
		const snapshotMenuBoxList = [...menuBoxList]
		const currentPageIndex =
			pageIndex + 1 > snapshotMenuBoxList.length ? 0 : pageIndex
		const newMenuBoxList: MenuBoxProps[] = snapshotMenuBoxList.map(
			(box, index) => {
				if (index === currentPageIndex) {
					box.display = true
				} else {
					box.display = false
				}
				return box
			},
		)
		setMenuBoxList(newMenuBoxList)
	}

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.menu, componentStyle]}>
				<h4 css={styles.menuTitle}>{t('menu.title')}</h4>

				<div
					id={'menuBoxContainer'}
					style={{
						position: 'relative',
						width: '100%',
						height: '100%',
						overflow: 'hidden',
					}}
				>
					{menuBoxList.map((props) => (
						<MenuBox
							key={props.first.item.code}
							first={props.first}
							firstNumber={props.firstNumber}
							second={props.second}
							secondNumber={props.secondNumber}
							display={props.display}
						/>
					))}
				</div>

				<div css={styles.notice}>{t('menu.notice')}</div>
			</div>
		</div>
	)
}

export default MenuDisplay
