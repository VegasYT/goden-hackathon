import { actorName } from '@entities/actor';
import { formatDateTime, formatEventTime } from '@shared/lib/datetime';
import { formatAmount } from '@shared/lib/quantity';

import type { Order, Shortage } from '../api/orders';

/** Состав заказа одной строкой: «Паста карбонара × 2, Омлет × 1». */
export const dishesText = (items: { dish_name?: string; portions: number }[]) =>
	items
		.map((item) => `${item.dish_name ?? 'Блюдо'} × ${item.portions}`)
		.join(', ');

/** Название недостающего продукта; продукта может уже не быть в каталоге. */
export const shortageName = (shortage: Shortage) =>
	shortage.product?.name ?? 'Продукт удалён из каталога';

/** Недостающие продукты одной строкой для строки истории. */
export const shortagesText = (order: Pick<Order, 'shortages'>) =>
	order.shortages.map(shortageName).join(', ');

/** Нужно, было и не хватило в единице продукта; без продукта единица неизвестна, и числа идут как есть. */
export function shortageAmounts(shortage: Shortage) {
	const missing = shortage.required_quantity - shortage.available_quantity;
	const format = (value: number) =>
		shortage.product
			? formatAmount(value, shortage.product.unit)
			: String(value);

	return {
		required: format(shortage.required_quantity),
		available: format(shortage.available_quantity),
		missing: format(missing),
	};
}

/** Время оформления заказа, как в строке истории: «15.10, 14:38». */
export const placedTime = (order: Pick<Order, 'placed_at'>) =>
	order.placed_at ? formatDateTime(order.placed_at) : '—';

/** Кто, когда и с каким комментарием отменил заказ, одной строкой. */
export const cancelledText = (
	order: Pick<Order, 'cancelled_by' | 'cancelled_at' | 'cancel_comment'>
) =>
	[
		`Отменил ${cancelledBy(order)}${order.cancelled_at ? `, ${formatEventTime(order.cancelled_at)}` : ''}`,
		order.cancel_comment && `«${order.cancel_comment}»`,
	]
		.filter(Boolean)
		.join(' · ');

/** Кто отменил заказ; у старого отменённого заказа автора может не быть. */
export const cancelledBy = (order: Pick<Order, 'cancelled_by'>) =>
	order.cancelled_by ? actorName(order.cancelled_by) : '—';
