/**
 * Firestoreのmenuコレクションのドキュメント型
 */
export type Menu = {
	/** ソートキー（文字列昇順でソート） */
	code: string
	/** メニュー名 */
	name: string
	/** 画像URL（SVG推奨） */
	image: string
}

/**
 * メニューアイテムとその番号（!order用）
 */
export type MenuItemWithNumber = Menu & {
	/** メニュー番号（1から始まる） */
	number: number
}
