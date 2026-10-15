import { queryOptions } from '@tanstack/react-query';

import { getData, pointPath, type components } from '@shared/api';
import { keepPointData, pointKey } from '@shared/lib/query';

export type ExpiringBatch = components['schemas']['ExpiringBatch'];
type ExpiringBatchPage = components['schemas']['ExpiringBatchPage'];
type StockStatus = components['schemas']['StockStatus'];

/** Порядки остатков, которые понимает сервер; минус — обратный. */
export const stockSorts = [
	'name',
	'-name',
	'nearest_expiry',
	'-nearest_expiry',
	'available',
	'-available',
] as const;

export type StockSort = (typeof stockSorts)[number];

/** Поиск и категория: по ним считаются и остатки, и сводка. */
export interface StockFilterParams {
	q?: string;
	category?: string;
}

export interface StockListParams extends StockFilterParams {
	status?: StockStatus;
	sort: StockSort;
	limit: number;
	offset: number;
}

/** Партии, срок которых истекает за `within_hours` часов или уже истёк, страницей. */
export interface ExpiringParams {
	within_hours: number;
	limit: number;
	offset: number;
}

/** Окно срочных сроков в часах: число на вкладке «Сроки» и счётчик сводки. */
export const urgentExpiringHours = 24;

/** Ключи запросов склада точки; весь склад сбрасывается по `stockKeys.all`. */
export const stockKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'stock'] as const,
	lists: (pointId: string) => [...stockKeys.all(pointId), 'list'] as const,
	list: (pointId: string, params: StockListParams) =>
		[...stockKeys.lists(pointId), params] as const,
	summaries: (pointId: string) =>
		[...stockKeys.all(pointId), 'summary'] as const,
	summary: (pointId: string, params: StockFilterParams) =>
		[...stockKeys.summaries(pointId), params] as const,
	batches: (pointId: string, productId: string) =>
		[...stockKeys.all(pointId), 'batches', productId] as const,
	movements: (pointId: string, productId: string) =>
		[...stockKeys.all(pointId), 'movements', productId] as const,
	expirings: (pointId: string) =>
		[...stockKeys.all(pointId), 'expiring'] as const,
	expiring: (pointId: string, params: ExpiringParams) =>
		[...stockKeys.expirings(pointId), params] as const,
};

/** Истекающие и просроченные партии точки от ближайшего срока. */
export function expiringBatchesQueryOptions(
	pointId: string,
	params: ExpiringParams
) {
	return queryOptions({
		queryKey: stockKeys.expiring(pointId, params),
		queryFn: getData<ExpiringBatchPage>(
			`${pointPath(pointId)}/batches/expiring`,
			params
		),
		placeholderData: keepPointData(pointId),
	});
}
