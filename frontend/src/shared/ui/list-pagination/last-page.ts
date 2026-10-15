import { useEffect } from 'react';

/** Последняя страница списка при заданном числе строк. */
export function lastPage(total: number, size: number): number {
	return Math.max(1, Math.ceil(total / size));
}

/** Страница из адреса оказалась за концом списка, например после смены фильтра в другой вкладке: открывает последнюю. */
export function useLastPageFallback(
	total: number | undefined,
	page: number,
	size: number,
	onPageChange: (page: number) => void
) {
	const last =
		total === undefined || total === 0 ? page : lastPage(total, size);

	useEffect(() => {
		if (page > last) {
			onPageChange(last);
		}
	}, [page, last, onPageChange]);
}
