import type { PageSize } from '@shared/lib/search-params';

import { useLastPageFallback } from './last-page';

interface PagedSearch {
	page: number;
	size: PageSize;
}

/** Страницы списка из адреса: `filter` меняет поиск и открывает первую страницу, `pagination` — пропсы `ListPagination`; страница за концом списка заменяется последней. */
export function usePagedSearch<S extends PagedSearch>(
	search: S,
	onSearchChange: (patch: Partial<S>) => void,
	total: number | undefined
) {
	const goTo = (page: number) => onSearchChange({ page } as Partial<S>);

	useLastPageFallback(total, search.page, search.size, goTo);

	return {
		filter: (patch: Partial<S>) => onSearchChange({ ...patch, page: 1 }),
		pagination: {
			page: search.page,
			size: search.size,
			onPageChange: goTo,
			onSizeChange: (size: PageSize) =>
				onSearchChange({ size, page: 1 } as Partial<S>),
		},
	};
}
