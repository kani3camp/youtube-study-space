type AppErrorProps = {
	error: unknown
}

export function AppError({ error }: AppErrorProps) {
	console.error(error)

	return (
		<section className="cardStack">
			<div className="card">
				<h2>エラーが発生しました</h2>
				<p className="mutedText">
					マイページ情報を表示できませんでした。時間を置いてもう一度お試しください。
				</p>
			</div>
		</section>
	)
}
