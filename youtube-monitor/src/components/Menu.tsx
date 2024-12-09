import { useTranslation } from 'next-i18next'
import { FC, useEffect, useMemo, useState } from 'react'
import Image from 'next/image'
import * as styles from '../styles/Menu.styles'
import { componentStyle, componentBackground } from '../styles/common.style'
import { firestoreMenuConverter, getFirebaseConfig } from '../lib/firestore'
import { collection, getFirestore, onSnapshot, orderBy, query } from 'firebase/firestore'
import { Menu } from '../types/api'
import { initializeApp } from 'firebase/app'

const checkImageExists = (url: string): Promise<boolean> =>
    new Promise((resolve) => {
        console.log('checkImageExists:', url)
        const img = new globalThis.Image()
        img.onload = () => resolve(true)
        img.onerror = () => resolve(false)
        img.src = url
    })

type MenuItemAndImage = {
    item: Menu
    imageUrl: string
}

const MenuDisplay: FC = () => {
    const { t } = useTranslation()

    const app = initializeApp(getFirebaseConfig())
    const db = getFirestore(app)

    const [latestMenuItems, setLatestMenuItems] = useState<Menu[]>([])
    const [menuItemAndImages, setMenuItems] = useState<MenuItemAndImage[]>([])
    const menuConverter = firestoreMenuConverter

    const menuQuery = useMemo(
        () => query(collection(db, 'menu'), orderBy('code', 'asc')).withConverter(menuConverter),
        [db, menuConverter]
    )
    useEffect(() => {
        const unsubscribe = onSnapshot(menuQuery, (querySnapshot) => {
            const menuItems: Menu[] = []
            querySnapshot.forEach((doc) => {
                menuItems.push(doc.data())
            })
            setLatestMenuItems(menuItems)
        })

        return () => {
            unsubscribe()
        }
    }, [menuQuery])

    useEffect(() => {
        console.log('MenuDisplay: useEffect')
        updateMenuItems()
    }, [latestMenuItems])

    const updateMenuItems = async () => {
        const menuItemAndImages = await Promise.all(
            latestMenuItems.map(async (item: Menu) => {
                let imageUrl = `/images/menu/${item.code}.svg`
                const imageExists = await checkImageExists(imageUrl)
                if (!imageExists) {
                    imageUrl = '/images/menu_default.svg'
                }
                return { item, imageUrl }
            })
        )
        setMenuItems(menuItemAndImages)
    }

    const imageList = menuItemAndImages.map((itemAndImage, i) => (
        <Image
            key={i}
            src={itemAndImage.imageUrl}
            alt='menu item'
            width={90}
            height={90}
            css={[styles.listItem, styles.image]}
        />
    ))

    const nameList = menuItemAndImages.map((itemAndImage, i) => (
        <div key={i} css={[styles.listItem, styles.name]}>
            {itemAndImage.item.display_name}
        </div>
    ))

    const commandList = menuItemAndImages.map((itemAndImage, i) => (
        <div key={i}>
            <span css={[styles.listItem, styles.commandCode]}>!order {i + 1}</span>
        </div>
    ))

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.menu, componentStyle]}>
                <h4 css={styles.menuTitle}>{t('menu.title')}</h4>

                <div css={styles.list}>{imageList}</div>
                <div css={styles.list}>{nameList}</div>
                <div css={styles.list}>{commandList}</div>

                <div css={styles.notice}>{t('menu.notice')}</div>
            </div>
        </div>
    )
}

export default MenuDisplay
