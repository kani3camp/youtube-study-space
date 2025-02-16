import { initializeApp } from 'firebase/app'
import {
	collection,
	getFirestore,
	onSnapshot,
	orderBy,
	query,
} from 'firebase/firestore'
import { useTranslation } from 'next-i18next'
import { type FC, useEffect, useMemo, useState, useCallback } from 'react'
import { useInterval } from '../lib/common'
import { firestoreMenuConverter, getFirebaseConfig } from '../lib/firestore'
import * as styles from '../styles/Menu.styles'
import { componentBackground, componentStyle } from '../styles/common.style'
import type { Menu } from '../types/api'
import MenuBox, { type MenuBoxProps } from './MenuBox'

export const checkImageExists = (url: string): Promise<boolean> =>
	new Promise((resolve) => {
		console.log('checkImageExists:', url)
		const img = new globalThis.Image()
		img.onload = () => resolve(true)
		img.onerror = () => resolve(false)
		img.src = url
	})

export type MenuItemAndImage = {
	item: Menu
	imageUrl: string
}

const MenuDisplay: FC = () => {
	const PAGING_INTERVAL_SEC = 5

	const { t } = useTranslation()

	const app = initializeApp(getFirebaseConfig())
	const db = getFirestore(app)

	const [latestMenuItems, setLatestMenuItems] = useState<Menu[]>([])
	const [menuBoxList, setMenuBoxList] = useState<MenuBoxProps[]>([])
	const [pageIndex, setPageIndex] = useState<number>(0)
	const menuConverter = firestoreMenuConverter

	const menuQuery = useMemo(
		() =>
			query(collection(db, 'menu'), orderBy('code', 'asc')).withConverter(
				menuConverter,
			),
		[db, menuConverter],
	)
	useEffect(() => {
		const unsubscribe = onSnapshot(menuQuery, (querySnapshot) => {
			const menuItems: Menu[] = []
			for (const doc of querySnapshot.docs) {
				menuItems.push(doc.data())
			}
			setLatestMenuItems(menuItems)
		})

		return () => {
			unsubscribe()
		}
	}, [menuQuery])

	const updateMenuItems = useCallback(async () => {
		const menuItemAndImages = await Promise.all(
			latestMenuItems.map(async (item: Menu) => {
				let imageUrl = `/images/menu/${item.code}.svg`
				const imageExists = await checkImageExists(imageUrl)
				if (!imageExists) {
					imageUrl = '/images/menu_default.svg'
				}
				return { item, imageUrl }
			}),
		)
		const menuBoxList: MenuBoxProps[] = []
		for (let i = 0; i < menuItemAndImages.length; i += 2) {
			const first = menuItemAndImages[i]
			const second = menuItemAndImages[i + 1] ? menuItemAndImages[i + 1] : null
			const firstNumber = i + 1
			const secondNumber = second ? i + 2 : null
			const display = i === 0
			menuBoxList.push({ first, firstNumber, second, secondNumber, display })
		}
		setMenuBoxList(menuBoxList)
		console.log(menuBoxList)
	}, [latestMenuItems])

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
		if (latestMenuItems.length > 0) {
			const newPageIndex =
				(pageIndex + 1) % Math.ceil(latestMenuItems.length / 2)
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
