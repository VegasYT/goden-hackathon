import type {
	InvalidateQueryFilters,
	QueryClient,
	QueryKey,
} from '@tanstack/react-query';

/**
 * Помечает устаревшими запросы с этими ключами; открытые на экране перечитываются, а уже начатый
 * запрос не обрывается и не уходит заново.
 */
export function invalidateKeys(
	queryClient: QueryClient,
	keys: readonly QueryKey[],
	filters?: Omit<InvalidateQueryFilters, 'queryKey'>
) {
	for (const queryKey of keys) {
		void queryClient.invalidateQueries(
			{ ...filters, queryKey },
			{ cancelRefetch: false }
		);
	}
}
