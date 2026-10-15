import { deliveryListQueryOptions } from '@entities/delivery';

/** Поставки точки с открытым карантином одной страницей: больше API не отдаёт. */
export function openQuarantineQueryOptions(pointId: string) {
	return deliveryListQueryOptions(pointId, {
		quarantine: 'open',
		sort: 'delivered_at',
		limit: 200,
		offset: 0,
	});
}
