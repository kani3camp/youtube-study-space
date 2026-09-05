import { useEffect, useState } from 'react'

export const SYNCHRONIZED_PAGE_UPDATE_INTERVAL_MS = 250

export const getSynchronizedPageIndex = (
	now: number,
	pageCount: number,
	intervalMs: number,
): number => {
	if (pageCount <= 0 || intervalMs <= 0) {
		return 0
	}
	return Math.floor(now / intervalMs) % pageCount
}

export const getFixedPageIndex = (
	page: string | string[] | undefined,
	pageCount: number,
): number | undefined => {
	if (pageCount <= 0 || page === undefined) {
		return undefined
	}
	const pageValue = Array.isArray(page) ? page[0] : page
	const pageNumber = Number(pageValue)
	if (
		!Number.isInteger(pageNumber) ||
		pageNumber < 1 ||
		pageNumber > pageCount
	) {
		return undefined
	}
	return pageNumber - 1
}

type UseSynchronizedPageParams = {
	pageCount: number
	intervalMs: number
	fixedPageIndex?: number
}

export const useSynchronizedPage = ({
	pageCount,
	intervalMs,
	fixedPageIndex,
}: UseSynchronizedPageParams): number => {
	const [currentPageIndex, setCurrentPageIndex] = useState(0)

	useEffect(() => {
		const updatePageIndex = () => {
			setCurrentPageIndex(
				fixedPageIndex ??
					getSynchronizedPageIndex(Date.now(), pageCount, intervalMs),
			)
		}

		updatePageIndex()
		const intervalId = window.setInterval(
			updatePageIndex,
			SYNCHRONIZED_PAGE_UPDATE_INTERVAL_MS,
		)
		return () => {
			window.clearInterval(intervalId)
		}
	}, [pageCount, intervalMs, fixedPageIndex])

	if (pageCount <= 0) {
		return 0
	}
	return Math.min(currentPageIndex, pageCount - 1)
}

export default useSynchronizedPage
