/**
 * ランダムにBGMを選ぶ。
 * @returns {Bgm} 選択したBGM
 */
export async function getCurrentRandomBgm(): Promise<string> {
	const res = await fetch('/api/bgm')
	const file = (await res.json()).file
	console.log(file)

	if (file) {
		return file
	}
	throw Error('no bgm file.')
}
