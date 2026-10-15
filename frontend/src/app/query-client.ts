import { QueryClient } from '@tanstack/react-query';

import { shouldRetry } from '@shared/api';
import { pointDataFreshMs } from '@shared/lib/query';

/** Клиент запросов приложения; в тестах `test: true` выключает повторы, чтобы ошибка была видна сразу. */
export function createQueryClient({ test = false }: { test?: boolean } = {}) {
	return new QueryClient({
		defaultOptions: {
			queries: {
				staleTime: pointDataFreshMs,
				retry: test ? false : shouldRetry,
			},
		},
	});
}
