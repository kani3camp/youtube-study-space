import * as fs from 'node:fs'
import * as path from 'node:path'
import * as puppeteer from 'puppeteer'
import { renderMenuBoardToHtml } from './components/MenuBoard'
import { fetchMenuItems, splitIntoPages } from './lib/firestore'

// 定数
const IMAGE_SIZE = 2048
const JPEG_QUALITY = 95
const ITEMS_PER_PAGE = 12
const IMAGE_LOAD_TIMEOUT = 30000 // 30秒
const OUTPUT_DIR = path.resolve(__dirname, '../output')

/**
 * 画像の読み込み完了を待つ
 */
async function waitForImages(page: puppeteer.Page): Promise<void> {
	await page.evaluate(() => {
		return new Promise<void>((resolve, reject) => {
			const images = Array.from(document.querySelectorAll('img'))

			if (images.length === 0) {
				resolve()
				return
			}

			let loadedCount = 0
			const totalCount = images.length

			const checkComplete = () => {
				loadedCount++
				if (loadedCount === totalCount) {
					resolve()
				}
			}

			for (const img of images) {
				if (img.complete) {
					checkComplete()
				} else {
					img.onload = checkComplete
					img.onerror = () => {
						reject(new Error(`画像の読み込みに失敗しました: ${img.src}`))
					}
				}
			}
		})
	})
}

/**
 * HTMLからJPEG画像を生成する
 */
async function generateImage(
	html: string,
	outputPath: string,
	browser: puppeteer.Browser,
): Promise<void> {
	const page = await browser.newPage()

	try {
		// ビューポートを設定
		await page.setViewport({
			width: IMAGE_SIZE,
			height: IMAGE_SIZE,
			deviceScaleFactor: 1,
		})

		// HTMLをセット
		console.log('  - ページを読み込み中...')
		await page.setContent(html, {
			waitUntil: 'networkidle0',
			timeout: IMAGE_LOAD_TIMEOUT,
		})

		// 全画像の読み込み完了を待つ
		console.log('  - 画像リソースを読み込み中...')
		await Promise.race([
			waitForImages(page),
			new Promise<void>((_, reject) =>
				setTimeout(
					() => reject(new Error('画像読み込みタイムアウト')),
					IMAGE_LOAD_TIMEOUT,
				),
			),
		])

		// スクリーンショットを撮影
		console.log('  - スクリーンショットを撮影中...')
		await page.screenshot({
			path: outputPath,
			type: 'jpeg',
			quality: JPEG_QUALITY,
			clip: {
				x: 0,
				y: 0,
				width: IMAGE_SIZE,
				height: IMAGE_SIZE,
			},
		})

		console.log(`  - 保存完了: ${path.basename(outputPath)}`)
	} finally {
		await page.close()
	}
}

/**
 * メイン処理
 */
async function main(): Promise<void> {
	console.log('=== Menu Image Generator ===\n')

	// 出力ディレクトリの作成
	if (!fs.existsSync(OUTPUT_DIR)) {
		fs.mkdirSync(OUTPUT_DIR, { recursive: true })
	}

	// メニューアイテムを取得
	console.log('Firestoreからメニューアイテムを取得中...')
	const menuItems = await fetchMenuItems()

	// ページに分割
	const pages = splitIntoPages(menuItems, ITEMS_PER_PAGE)
	const totalPages = pages.length
	console.log(`${totalPages}ページに分割しました\n`)

	// Puppeteerブラウザを起動
	console.log('ブラウザを起動中...')
	const browser = await puppeteer.launch({
		headless: 'new', // v21系では新しいHeadlessモードを明示的に指定
		args: ['--no-sandbox', '--disable-setuid-sandbox'],
	})
	console.log('ブラウザ起動完了')

	try {
		// 各ページを画像に変換
		for (let i = 0; i < pages.length; i++) {
			const pageNumber = i + 1
			const pageItems = pages[i]

			console.log(
				`\nページ ${pageNumber}/${totalPages} を生成中... (${pageItems.length}アイテム)`,
			)

			// HTMLを生成
			const html = renderMenuBoardToHtml(pageItems, pageNumber, totalPages)

			// ファイル名を決定
			const fileName = totalPages === 1 ? 'menu.jpg' : `menu_${pageNumber}.jpg`
			const outputPath = path.join(OUTPUT_DIR, fileName)

			// 画像を生成
			await generateImage(html, outputPath, browser)
		}

		console.log('\nブラウザを終了中...')
	} finally {
		// Puppeteerのclose()でクリーンアップを行う
		await browser.close()
	}

	console.log('\n=== 完了 ===')
	console.log(`出力先: ${OUTPUT_DIR}`)
}

// エントリポイント
main().catch((error) => {
	console.error('エラーが発生しました:', error.message)
	process.exit(1)
})
