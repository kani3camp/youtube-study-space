import * as path from 'node:path'
import * as dotenv from 'dotenv'
import * as admin from 'firebase-admin'
import type { MenuItemWithNumber } from '../types'

// .envファイルを読み込み
dotenv.config()

let initialized = false

/**
 * Firebase Adminを初期化する
 */
export function initializeFirebase(): void {
	if (initialized) {
		return
	}

	const credentialPath = process.env.GOOGLE_APPLICATION_CREDENTIALS
	if (!credentialPath) {
		throw new Error(
			'GOOGLE_APPLICATION_CREDENTIALS環境変数が設定されていません。.envファイルを確認してください。',
		)
	}

	const absolutePath = path.isAbsolute(credentialPath)
		? credentialPath
		: path.resolve(process.cwd(), credentialPath)

	admin.initializeApp({
		credential: admin.credential.cert(absolutePath),
	})

	initialized = true
	console.log('Firebase Admin initialized successfully')
}

/**
 * Firestoreからmenuコレクションを取得する
 * codeの文字列昇順でソート（現行システムと同様）
 * @returns メニューアイテムの配列（番号付き）
 */
export async function fetchMenuItems(): Promise<MenuItemWithNumber[]> {
	initializeFirebase()

	const db = admin.firestore()
	const menuCollection = db.collection('menu')

	// codeの文字列昇順でソート（Firestoreのクエリで実行）
	const snapshot = await menuCollection.orderBy('code', 'asc').get()

	if (snapshot.empty) {
		throw new Error('menuコレクションにドキュメントがありません')
	}

	const menuItems: MenuItemWithNumber[] = []

	snapshot.forEach((doc) => {
		const data = doc.data()

		// imageフィールドの検証
		if (
			!data.image ||
			typeof data.image !== 'string' ||
			data.image.trim() === ''
		) {
			throw new Error(
				`メニュー "${data.name || doc.id}" (code: ${data.code || 'undefined'}) のimageフィールドが欠損しています`,
			)
		}

		// imageフィールドが有効なURLか検証
		try {
			new URL(data.image)
		} catch {
			throw new Error(
				`メニュー "${data.name || doc.id}" (code: ${data.code || 'undefined'}) のimageフィールドが有効なURLではありません: ${data.image}`,
			)
		}

		menuItems.push({
			code: data.code || '',
			name: data.name || '',
			image: data.image,
			number: menuItems.length + 1, // 1から始まる番号
		})
	})

	console.log(`${menuItems.length}件のメニューアイテムを取得しました`)
	return menuItems
}

/**
 * メニューアイテムをページごとに分割する
 * @param items メニューアイテムの配列
 * @param itemsPerPage 1ページあたりの最大アイテム数
 * @returns ページごとに分割されたメニューアイテムの配列
 */
export function splitIntoPages(
	items: MenuItemWithNumber[],
	itemsPerPage = 16,
): MenuItemWithNumber[][] {
	const pages: MenuItemWithNumber[][] = []

	for (let i = 0; i < items.length; i += itemsPerPage) {
		pages.push(items.slice(i, i + itemsPerPage))
	}

	return pages
}
