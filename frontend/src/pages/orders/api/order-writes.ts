import { useMutation, useQueryClient } from '@tanstack/react-query';

import { dishKeys, menuQueryOptions, type DishRef } from '@entities/dish';
import { ApiError } from '@shared/api';
import { useIdempotencyKey } from '@shared/lib/idempotency';

import { droppedDishes } from '../model/order-form';
import { refreshAfterOrder } from '../model/refresh';
import {
	cancelOrder,
	orderKeys,
	placeOrder,
	type NewOrder,
	type Order,
	type OrderPage,
} from './orders';

interface OrderAttempt {
	body: NewOrder;
	/** Блюда заказа с названиями: по ним называется блюдо, которое выключили на точке. */
	dishes: DishRef[];
}

interface PlaceOrderHandlers {
	onPlaced: (order: Order) => void;
	/** Блюда заказа, которые выключили на точке, пока он заполнялся. */
	onDropped: (dishes: DishRef[]) => void;
}

/** Оформляет заказ; если блюдо выключили на точке, перечитывает меню и передаёт выключенные блюда в `onDropped`. */
export function usePlaceOrder(
	pointId: string,
	{ onPlaced, onDropped }: PlaceOrderHandlers
) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: ({ body }: OrderAttempt) =>
			placeOrder(pointId, body, keyFor(body)),
		onSuccess: (order) => {
			refreshAfterOrder(queryClient, pointId);
			onPlaced(order);
		},
		onError: async (error, { dishes }) => {
			// Блюдо выключили, пока заполнялся заказ: сервер называет его только идентификатором.
			if (
				!(error instanceof ApiError) ||
				error.status !== 422 ||
				error.type !== 'unknown-reference'
			) {
				return;
			}

			await queryClient.invalidateQueries({ queryKey: dishKeys.all(pointId) });
			const fresh = await queryClient.fetchQuery(menuQueryOptions(pointId));
			const dropped = droppedDishes(dishes, fresh);

			if (dropped.length > 0) {
				onDropped(dropped);
				mutation.reset();
			}
		},
	});

	return mutation;
}

/** Отменяет заказ: ответ сразу заменяет заказ в истории, а после отказа сервера история перечитывается. */
export function useCancelOrder(pointId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: ({ orderId, comment }: { orderId: string; comment: string }) =>
			cancelOrder(pointId, orderId, comment, keyFor({ orderId, comment })),
		onSuccess: (order) => {
			queryClient.setQueriesData<OrderPage>(
				{ queryKey: orderKeys.lists(pointId) },
				(page) =>
					page && {
						...page,
						items: page.items.map((item) =>
							item.id === order.id ? order : item
						),
					}
			);
			refreshAfterOrder(queryClient, pointId);
		},
		onError: () =>
			queryClient.invalidateQueries({ queryKey: orderKeys.lists(pointId) }),
	});
}
