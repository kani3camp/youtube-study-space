/**
 * ランダムにBGMを選ぶ．
 * @return {string} 現在のBGMファイルのパス
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

export type Bgm = {
  title: string
  artist: string
  file: string
  forPart: string[]
}

export const AllPartType = ['AllPartType']

