/**
 * ポモドーロタイマー用計算関数
 * @param {number} elapsedSeconds 経過時間（秒）
 * @return {[number, number, boolean]} 今の状態の残り時間, プログレスバーの残りパーセント, 作業中かどうか
 */
export const calcPomodoroRemaining = (elapsedSeconds: number) => {
    const elapsedMinutes = Math.floor(elapsedSeconds / 60)

    // elapsedMinutesを超えるまで25と5を交互に足していく
    let sumMinutes = 0
    let isStudying = false
    while (sumMinutes <= elapsedMinutes) {
        isStudying = !isStudying

        if (isStudying) {
            sumMinutes += 25
        } else {
            sumMinutes += 5
        }
    }

    // 残り時間・プログレスバー用の進捗パーセントを求める
    const remainingSeconds: number = sumMinutes * 60 - elapsedSeconds
    const remainingPercentage: number = isStudying
        ? (remainingSeconds / (25 * 60)) * 100
        : (remainingSeconds / (5 * 60)) * 100
    return [remainingSeconds, remainingPercentage, isStudying]
}

export const calcNumberOfPomodoroRounds = (elapsedSeconds: number) => {
    return Math.ceil(elapsedSeconds / 60 / 30)
}
