import { useTranslation } from 'next-i18next'
import { FC, useState } from 'react'
import * as styles from '../styles/Menu.styles'
import { componentStyle, componentBackground } from '../styles/common.style'
import { firestoreMenuConverter, getFirebaseConfig } from '../lib/firestore'
import { collection, getFirestore, onSnapshot, orderBy, query } from 'firebase/firestore'
import { Menu } from '../types/api'
import { initializeApp } from 'firebase/app'

const MenuDisplay: FC = () => {
    const { t } = useTranslation()

    const app = initializeApp(getFirebaseConfig())
    const db = getFirestore(app)

    const [latestMenuItems, setLatestMenuItems] = useState<Menu[]>([])
    const menuConverter = firestoreMenuConverter

    const menuQuery = query(collection(db, 'menu'), orderBy('code', 'asc')).withConverter(
        menuConverter
    )
    onSnapshot(menuQuery, (querySnapshot) => {
        const menuItems: Menu[] = []
        querySnapshot.forEach((doc) => {
            menuItems.push(doc.data())
        })
        setLatestMenuItems(menuItems)
    })

    const itemDivList = latestMenuItems.map((item) => (
        <div key={item.code} css={styles.itemBody}>
            {item.code}
        </div>
    ))

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.menu, componentStyle]}>
                <h4 css={styles.menuTitle}>{t('menu.title')}</h4>

                <div css={styles.menuBody}>{itemDivList}</div>

                <div css={styles.notice}>{t('menu.notice')}</div>
            </div>
        </div>
    )
}

export default MenuDisplay
