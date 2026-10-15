import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	getData,
	idempotencyHeader,
	pointPath,
	type components,
} from '@shared/api';
import { keepPointData, pointKey } from '@shared/lib/query';

export type Delivery = components['schemas']['Delivery'];
export type DeliveryItem = components['schemas']['DeliveryItem'];
export type DeliveryPage = components['schemas']['DeliveryPage'];
export type DeliverySummary = components['schemas']['DeliverySummary'];
export type DeliveryHistory = components['schemas']['DeliveryHistory'];
export type DeliveryHistoryEntry =
	components['schemas']['DeliveryHistoryEntry'];
export type NewDelivery = components['schemas']['NewDelivery'];

export type DeliverySort = 'delivered_at' | '-delivered_at';

export interface DeliveryListParams {
	status?: Delivery['status'];
	/** `open` — только поставки с позицией в карантине без решения. */
	quarantine?: 'open';
	/** Начало номера поставки. */
	number?: string;
	sort: DeliverySort;
	limit: number;
	offset: number;
}

/** Ключи запросов поставок точки; весь раздел точки сбрасывается по `deliveryKeys.all`. */
export const deliveryKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'deliveries'] as const,
	lists: (pointId: string) => [...deliveryKeys.all(pointId), 'list'] as const,
	list: (pointId: string, params: DeliveryListParams) =>
		[...deliveryKeys.lists(pointId), params] as const,
	summary: (pointId: string) =>
		[...deliveryKeys.all(pointId), 'summary'] as const,
	details: (pointId: string) =>
		[...deliveryKeys.all(pointId), 'detail'] as const,
	detail: (pointId: string, deliveryId: string) =>
		[...deliveryKeys.details(pointId), deliveryId] as const,
	histories: (pointId: string) =>
		[...deliveryKeys.all(pointId), 'history'] as const,
	history: (pointId: string, deliveryId: string) =>
		[...deliveryKeys.histories(pointId), deliveryId] as const,
};

/** Адрес поставок точки в API: список и регистрация. */
const deliveriesPath = (pointId: string) => `${pointPath(pointId)}/deliveries`;

/** Адрес поставки в API, от него строятся адреса её действий. */
export const deliveryPath = (pointId: string, deliveryId: string) =>
	`${deliveriesPath(pointId)}/${encodeURIComponent(deliveryId)}`;

/** Страница списка поставок; при смене параметров на той же точке показываются прежние строки, пока не придут новые. */
export function deliveryListQueryOptions(
	pointId: string,
	params: DeliveryListParams
) {
	return queryOptions({
		queryKey: deliveryKeys.list(pointId, params),
		queryFn: getData<DeliveryPage>(deliveriesPath(pointId), params),
		// Строки другой точки не показываем: ссылки в них вели бы на чужие поставки.
		placeholderData: keepPointData(pointId),
	});
}

export function deliverySummaryQueryOptions(pointId: string) {
	return queryOptions({
		queryKey: deliveryKeys.summary(pointId),
		queryFn: getData<DeliverySummary>(`${deliveriesPath(pointId)}/summary`),
	});
}

export function deliveryQueryOptions(pointId: string, deliveryId: string) {
	return queryOptions({
		queryKey: deliveryKeys.detail(pointId, deliveryId),
		queryFn: getData<Delivery>(deliveryPath(pointId, deliveryId)),
	});
}

/** История поставки: события её потока по порядку, у каждого — состояние поставки сразу после него. */
export function deliveryHistoryQueryOptions(
	pointId: string,
	deliveryId: string
) {
	return queryOptions({
		queryKey: deliveryKeys.history(pointId, deliveryId),
		queryFn: getData<DeliveryHistory>(
			`${deliveryPath(pointId, deliveryId)}/history`
		),
	});
}

/** Регистрирует поставку; повтор с тем же ключом возвращает уже созданную. */
export async function registerDelivery(
	pointId: string,
	body: NewDelivery,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Delivery>(
			deliveriesPath(pointId),
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
