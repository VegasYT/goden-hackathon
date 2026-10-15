import type { UseQueryResult } from '@tanstack/react-query';

/** Состояние блока сводки по его запросам: ошибка, если упал хоть один. */
export interface BlockQuery {
	isFetching: boolean;
	error: Error | null;
	/** Повторяет только упавшие запросы. */
	refetch: () => void;
}

export function blockQuery(...queries: UseQueryResult[]): BlockQuery {
	return {
		isFetching: queries.some((query) => query.isFetching),
		error: queries.find((query) => query.error)?.error ?? null,
		refetch: () => {
			for (const query of queries) {
				if (query.isError) {
					void query.refetch();
				}
			}
		},
	};
}
