import {
	hashKey,
	type QueryClient,
	type QueryKey,
} from '@tanstack/react-query';

import { invalidateKeys } from './invalidate-keys';

/** Через сколько после записи перечитывать список: он строится проекцией и отстаёт от записи. */
export const listRefreshDelayMs = 1000;

/** Дольше этого непрерывная серия вызовов перечитывание не откладывает. */
const maxWaitMs = 3000;

type Pending = Map<string, { timer: number; since: number }>;

const pending = new WeakMap<QueryClient, Pending>();

/**
 * Помечает запросы с ключом устаревшими через `delayMs`. Повторный вызов с тем же ключом
 * до срока переносит срок, но не дальше `maxWaitMs` от первого вызова серии.
 */
export function invalidateLater(
	queryClient: QueryClient,
	queryKey: QueryKey,
	delayMs = listRefreshDelayMs
) {
	const timers: Pending = pending.get(queryClient) ?? new Map();
	const hash = hashKey(queryKey);
	const now = Date.now();
	const previous = timers.get(hash);
	const since = previous?.since ?? now;

	pending.set(queryClient, timers);
	window.clearTimeout(previous?.timer);
	timers.set(hash, {
		since,
		timer: window.setTimeout(
			() => {
				timers.delete(hash);
				invalidateKeys(queryClient, [queryKey]);
			},
			Math.min(delayMs, since + maxWaitMs - now)
		),
	});
}
