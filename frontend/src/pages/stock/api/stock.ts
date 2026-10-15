import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';

import {
	stockKeys,
	type StockFilterParams,
	type StockListParams,
} from '@entities/stock';
import {
	apiClient,
	getData,
	idempotencyHeader,
	pointPath,
	type components,
} from '@shared/api';
import { keepPointData } from '@shared/lib/query';

export type StockLevel = components['schemas']['StockLevel'];
type StockPage = components['schemas']['StockPage'];
export type StockSummary = components['schemas']['StockSummary'];
export type Batch = components['schemas']['Batch'];
type BatchList = components['schemas']['BatchList'];
export type Movement = components['schemas']['Movement'];
type MovementPage = components['schemas']['MovementPage'];
export type WriteOff = components['schemas']['WriteOff'];
export type Adjustment = components['schemas']['Adjustment'];

const batchLimit = 200;
const movementPage = 20;

const batchPath = (pointId: string, batchId: string) =>
	`${pointPath(pointId)}/batches/${encodeURIComponent(batchId)}`;

const productPath = (pointId: string, productId: string) =>
	`${pointPath(pointId)}/products/${encodeURIComponent(productId)}`;

/** Страница остатков; пока идёт новый запрос той же точки, видны прежние строки. */
export function stockListQueryOptions(
	pointId: string,
	params: StockListParams
) {
	return queryOptions({
		queryKey: stockKeys.list(pointId, params),
		queryFn: getData<StockPage>(`${pointPath(pointId)}/stock`, params),
		placeholderData: keepPointData(pointId),
	});
}

/** Число продуктов по статусам с учётом поиска и категории. */
export function stockSummaryQueryOptions(
	pointId: string,
	params: StockFilterParams
) {
	return queryOptions({
		queryKey: stockKeys.summary(pointId, params),
		queryFn: getData<StockSummary>(
			`${pointPath(pointId)}/stock/summary`,
			params
		),
		placeholderData: keepPointData(pointId),
	});
}

/** Партии продукта с остатком в порядке расхода. */
export function batchesQueryOptions(pointId: string, productId: string) {
	return queryOptions({
		queryKey: stockKeys.batches(pointId, productId),
		queryFn: async (context) =>
			(
				await getData<BatchList>(`${productPath(pointId, productId)}/batches`, {
					in_stock: true,
					limit: batchLimit,
				})(context)
			).items,
	});
}

/** Движения по продукту от новых к старым, страницами для «Показать ещё». */
export function movementsQueryOptions(pointId: string, productId: string) {
	return infiniteQueryOptions({
		queryKey: stockKeys.movements(pointId, productId),
		queryFn: (context) =>
			getData<MovementPage>(`${productPath(pointId, productId)}/movements`, {
				limit: movementPage,
				offset: context.pageParam,
			})(context),
		initialPageParam: 0,
		getNextPageParam: (last, pages) => {
			const loaded = pages.reduce((sum, page) => sum + page.items.length, 0);

			return loaded < last.total ? loaded : undefined;
		},
	});
}

export async function writeOffBatch(
	pointId: string,
	batchId: string,
	body: WriteOff,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Batch>(
			`${batchPath(pointId, batchId)}/write-offs`,
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function adjustBatch(
	pointId: string,
	batchId: string,
	body: Adjustment,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Batch>(
			`${batchPath(pointId, batchId)}/adjustments`,
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
