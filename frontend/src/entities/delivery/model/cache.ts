import type { QueryClient } from '@tanstack/react-query';

import { invalidateKeys, invalidateLater } from '@shared/lib/query';

import {
	deliveryKeys,
	type Delivery,
	type DeliveryPage,
} from '../api/delivery-queries';

/** Кладёт свежую поставку из ответа записи в кэш карточки и в строки закэшированных списков точки. */
export function storeDelivery(
	queryClient: QueryClient,
	pointId: string,
	delivery: Delivery
) {
	queryClient.setQueryData(deliveryKeys.detail(pointId, delivery.id), delivery);
	queryClient.setQueriesData<DeliveryPage>(
		{ queryKey: deliveryKeys.lists(pointId) },
		(page) =>
			page && {
				...page,
				items: page.items.map((item) =>
					item.id === delivery.id ? delivery : item
				),
			}
	);
	// История читается из потока событий и от записи не отстаёт, поэтому сбрасывается сразу.
	void queryClient.invalidateQueries({
		queryKey: deliveryKeys.history(pointId, delivery.id),
	});

	scheduleListRefresh(queryClient, pointId);
}

/** Перечитывает поставки точки после сигнала с сервера: карточки и историю сразу, списки и сводку с задержкой. */
export function refreshDeliveries(queryClient: QueryClient, pointId: string) {
	invalidateKeys(queryClient, [
		deliveryKeys.details(pointId),
		deliveryKeys.histories(pointId),
	]);

	scheduleListRefresh(queryClient, pointId);
}

// Записи подряд, например несколько решений по карантину, перечитывают списки один раз.
function scheduleListRefresh(queryClient: QueryClient, pointId: string) {
	invalidateLater(queryClient, deliveryKeys.lists(pointId));
	invalidateLater(queryClient, deliveryKeys.summary(pointId));
}
