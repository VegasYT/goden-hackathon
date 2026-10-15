import type { QueryClient } from '@tanstack/react-query';

import { dishKeys, hotDishKeys } from '@entities/dish';
import { invalidateKeys, invalidateLater } from '@shared/lib/query';

import { orderKeys } from '../api/orders';

/** Перечитывает историю заказов точки с задержкой: она строится на сервере и отстаёт от записи. */
export function refreshOrders(queryClient: QueryClient, pointId: string) {
	invalidateLater(queryClient, orderKeys.lists(pointId));
}

/** После своего заказа: меню и горячие предложения сразу, история с задержкой. */
export function refreshAfterOrder(queryClient: QueryClient, pointId: string) {
	invalidateKeys(queryClient, [
		dishKeys.all(pointId),
		hotDishKeys.all(pointId),
	]);

	refreshOrders(queryClient, pointId);
}
