type SummaryCardProps = {
	label: string
	value: string
}

export function SummaryCard({ label, value }: SummaryCardProps) {
	return (
		<section className="card summaryCard">
			<p className="cardLabel">{label}</p>
			<p className="summaryValue">{value}</p>
		</section>
	)
}
