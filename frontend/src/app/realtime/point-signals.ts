import type { QueryClient, QueryKey } from '@tanstack/react-query';

import { refreshDeliveries } from '@entities/delivery';
import { dishKeys, hotDishKeys } from '@entities/dish';
import { findPoint, meQueryOptions } from '@entities/profile';
import { stockKeys } from '@entities/stock';
import { analyticsKeys } from '@pages/analytics';
import { refreshCatalog } from '@pages/catalog';
import { refreshOrders } from '@pages/orders';
import { recommendationKeys } from '@pages/stock';
import type { components } from '@shared/api';
import {
	invalidateKeys,
	invalidateLater,
	pointDataFreshMs,
	pointKey,
} from '@shared/lib/query';

type Category = components['schemas']['RealtimeCategory'];
type Refresh = (queryClient: QueryClient, pointId: string) => void;

const invalidate =
	(...keysOf: ((pointId: string) => QueryKey)[]): Refresh =>
	(queryClient, pointId) =>
		invalidateKeys(
			queryClient,
			keysOf.map((keyOf) => keyOf(pointId))
		);

const both =
	(first: Refresh, second: Refresh): Refresh =>
	(queryClient, pointId) => {
		first(queryClient, pointId);
		second(queryClient, pointId);
	};

// Аналитика считается тяжёлыми агрегатами, поэтому серия сигналов перечитывает её один раз.
const refreshAnalytics: Refresh = (queryClient, pointId) =>
	invalidateLater(queryClient, analyticsKeys.all(pointId));

// Истекающие партии показывает склад, поэтому `stock` и `expiry` перечитывают его одним действием.
const refreshStock = both(invalidate(stockKeys.all), refreshAnalytics);

// Каталог общий на компанию: компания точки берётся из профиля в кэше.
const refreshCatalogOf: Refresh = (queryClient, pointId) => {
	const profile = queryClient.getQueryData(meQueryOptions.queryKey);
	const point = profile && findPoint(profile, pointId);

	if (point) {
		refreshCatalog(queryClient, point.company_id, pointId);
	}
};

// Новый раздел данных точки получает здесь строку, если его меняют сигналы потока.
const refreshByCategory: Record<Category, Refresh> = {
	stock: refreshStock,
	expiry: refreshStock,
	'stop-list': both(invalidate(dishKeys.all), refreshAnalytics),
	recommendations: invalidate(recommendationKeys.all, hotDishKeys.all),
	deliveries: refreshDeliveries,
	orders: both(refreshOrders, refreshAnalytics),
	catalog: refreshCatalogOf,
};

/** Помечает устаревшими данные точки по категориям сигнала; незнакомые категории пропускает. */
export function refreshCategories(
	queryClient: QueryClient,
	pointId: string,
	categories: readonly string[]
) {
	const refreshes = new Set(
		categories
			.filter((category) => Object.hasOwn(refreshByCategory, category))
			.map((category) => refreshByCategory[category as Category])
	);

	for (const refresh of refreshes) {
		refresh(queryClient, pointId);
	}
}

/**
 * Перечитывает данные точки после `sync`: после переподключения все, после первого подключения
 * только старше `pointDataFreshMs`.
 */
export function refreshAfterSync(
	queryClient: QueryClient,
	pointId: string,
	{ reconnect }: { reconnect: boolean }
) {
	const queryKey = pointKey(pointId);

	if (reconnect) {
		void queryClient.invalidateQueries({ queryKey });
		return;
	}

	const freshSince = Date.now() - pointDataFreshMs;

	// Запросы загрузчиков роута ещё могут идти, их не перезапускаем.
	invalidateKeys(queryClient, [queryKey], {
		predicate: (query) => query.state.dataUpdatedAt < freshSince,
	});
}
