import type { QueryClient } from '@tanstack/react-query';

import {
	deliveryListQueryOptions,
	deliverySummaryQueryOptions,
} from '@entities/delivery';
import {
	defaultHotHorizonHours,
	hotDishesQueryOptions,
	menuQueryOptions,
} from '@entities/dish';
import type { ProfilePoint } from '@entities/profile';
import {
	expiringBatchesQueryOptions,
	urgentExpiringHours,
} from '@entities/stock';

import { dashboardBlocks } from '../model/blocks';

/** Сколько ближайших партий и горячих предложений видно на сводке. */
const expiringRows = 6;
const hotRows = 3;

export const expiringQueryOptions = (pointId: string) =>
	expiringBatchesQueryOptions(pointId, {
		within_hours: urgentExpiringHours,
		limit: expiringRows,
		offset: 0,
	});

export const hotQueryOptions = (pointId: string) =>
	hotDishesQueryOptions(pointId, {
		horizon_hours: defaultHotHorizonHours,
		limit: hotRows,
		offset: 0,
	});

/** Последняя поставка, которая ждёт приёмки; `total` — сколько их всего. */
export const pendingQueryOptions = (pointId: string) =>
	deliveryListQueryOptions(pointId, {
		status: 'registered',
		sort: '-delivered_at',
		limit: 1,
		offset: 0,
	});

/** Поставки с открытым карантином: нужно только их число. */
export const quarantineDeliveriesQueryOptions = (pointId: string) =>
	deliveryListQueryOptions(pointId, {
		quarantine: 'open',
		sort: '-delivered_at',
		limit: 1,
		offset: 0,
	});

/** Начинает загрузку блоков, которые пользователь увидит на сводке точки. */
export function prefetchDashboard(
	queryClient: QueryClient,
	point: ProfilePoint
) {
	const blocks = dashboardBlocks(point);

	if (blocks.expiring) {
		void queryClient.prefetchQuery(expiringQueryOptions(point.id));
	}

	if (blocks.stopList) {
		void queryClient.prefetchQuery(menuQueryOptions(point.id));
	}

	if (blocks.hot) {
		void queryClient.prefetchQuery(hotQueryOptions(point.id));
	}

	if (blocks.deliveries) {
		void queryClient.prefetchQuery(pendingQueryOptions(point.id));
		void queryClient.prefetchQuery(deliverySummaryQueryOptions(point.id));
		void queryClient.prefetchQuery(quarantineDeliveriesQueryOptions(point.id));
	}
}
