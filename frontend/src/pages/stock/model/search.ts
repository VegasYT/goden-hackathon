import * as z from 'zod/mini';

import {
	stockSorts,
	type ExpiringParams,
	type StockFilterParams,
	type StockListParams,
} from '@entities/stock';
import {
	hourWindowField,
	pageFields,
	textField,
	toPageParams,
	withDefault,
} from '@shared/lib/search-params';

import type { RecommendationPage } from '../api/recommendations';

/** Общее для вкладок склада: продукт, история которого открыта в боковой панели. */
export const stockLayoutSearchSchema = z.object({
	history: z.catch(z.optional(z.string()), undefined),
});

/** Состояние вкладки «Остатки» в адресе. */
export const stockSearchSchema = z.object({
	q: textField(),
	category: textField(),
	status: withDefault(
		z.enum(['all', 'expiring', 'expired', 'out_of_stock']),
		'all'
	),
	sort: withDefault(z.enum(stockSorts), 'name'),
	...pageFields,
});

export type StockSearch = z.output<typeof stockSearchSchema>;
export type StockStatusFilter = StockSearch['status'];

/** Значения по умолчанию: их нет в адресе. */
export const stockSearchDefaults = stockSearchSchema.parse({});

export function toFilterParams(search: StockSearch): StockFilterParams {
	return {
		q: search.q.trim() || undefined,
		category: search.category || undefined,
	};
}

export function toStockParams(search: StockSearch): StockListParams {
	return {
		...toFilterParams(search),
		status: search.status === 'all' ? undefined : search.status,
		sort: search.sort,
		...toPageParams(search),
	};
}

/** Поиск, категория или статус сужают список. */
export function isFiltered(search: StockSearch): boolean {
	return (
		search.q.trim() !== '' || search.category !== '' || search.status !== 'all'
	);
}

/** Состояние вкладки «Сроки» в адресе. */
export const expiringSearchSchema = z.object({
	within: hourWindowField(72),
	...pageFields,
});

export type ExpiringSearch = z.output<typeof expiringSearchSchema>;

export const expiringSearchDefaults = expiringSearchSchema.parse({});

export function toExpiringParams(search: ExpiringSearch): ExpiringParams {
	return {
		within_hours: search.within,
		...toPageParams(search),
	};
}

/** Состояние вкладки «Пора заказать» в адресе. */
export const reorderSearchSchema = z.object(pageFields);

export type ReorderSearch = z.output<typeof reorderSearchSchema>;

export const reorderSearchDefaults = reorderSearchSchema.parse({});

export function toRecommendationPage(
	search: ReorderSearch
): RecommendationPage {
	return toPageParams(search);
}
