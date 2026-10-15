import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	getData,
	idempotencyHeader,
	pointPath,
	type components,
} from '@shared/api';
import { keepPointData, pointKey } from '@shared/lib/query';

export type Order = components['schemas']['Order'];
export type Shortage = components['schemas']['Shortage'];
export type NewOrder = components['schemas']['NewOrder'];
export type OrderPage = components['schemas']['OrderPage'];

export interface OrderListParams {
	status?: Order['status'];
	limit: number;
	offset: number;
}

/** Ключи заказов точки; все сбрасываются по `orderKeys.all`. */
export const orderKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'orders'] as const,
	lists: (pointId: string) => [...orderKeys.all(pointId), 'list'] as const,
	list: (pointId: string, params: OrderListParams) =>
		[...orderKeys.lists(pointId), params] as const,
};

const ordersPath = (pointId: string) => `${pointPath(pointId)}/orders`;

/** Страница истории заказов от новых к старым. */
export function orderListQueryOptions(
	pointId: string,
	params: OrderListParams
) {
	return queryOptions({
		queryKey: orderKeys.list(pointId, params),
		queryFn: getData<OrderPage>(ordersPath(pointId), params),
		placeholderData: keepPointData(pointId),
	});
}

/** Оформляет заказ; отказ из-за нехватки приходит заказом со статусом `rejected`. */
export async function placeOrder(
	pointId: string,
	body: NewOrder,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Order>(
			ordersPath(pointId),
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

/** Отменяет принятый заказ и возвращает его продукты на склад; пустой комментарий не отправляется. */
export async function cancelOrder(
	pointId: string,
	orderId: string,
	comment: string,
	idempotencyKey: string
) {
	const text = comment.trim();

	return (
		await apiClient.post<Order>(
			`${ordersPath(pointId)}/${orderId}/cancel`,
			text ? { comment: text } : undefined,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
