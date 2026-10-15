import { isExpired } from '@shared/lib/expiry';
import { countOf } from '@shared/lib/plural';
import type { QuantityUnit } from '@shared/lib/quantity';

import type { Delivery, DeliveryItem } from '../api/delivery-queries';

/** Сколько позиций идёт на склад, отклонено и в открытом карантине. */
export interface ItemOutcomes {
	toStock: number;
	rejected: number;
	quarantined: number;
}

/** Распределение количества позиции в полях API. */
export type Distribution = Pick<
	DeliveryItem,
	'accepted_quantity' | 'rejected_quantity' | 'quarantined_quantity'
>;

export function itemOutcomes(items: Distribution[]): ItemOutcomes {
	return {
		toStock: items.filter((item) => item.accepted_quantity > 0).length,
		rejected: items.filter((item) => item.rejected_quantity > 0).length,
		quarantined: items.filter((item) => item.quarantined_quantity > 0).length,
	};
}

/** Позиции ожидающей поставки, срок которых истёк к моменту `now`. */
export function expiredItems(delivery: Delivery, now: Date): number {
	return delivery.items.filter((item) => isExpired(item.expires_at, now))
		.length;
}

export function batchCount(delivery: Delivery): number {
	return delivery.items.reduce((sum, item) => sum + item.batch_ids.length, 0);
}

/** «1 позиция», «3 позиции», «8 позиций». */
export const positionsCount = (count: number) =>
	countOf(count, 'позиция', 'позиции', 'позиций');

/** «Поставка № Н-4512 зарегистрирована · 4 позиции». */
export const registeredMessage = (number: string, positions: number) =>
	`Поставка № ${number} зарегистрирована · ${positionsCount(positions)}`;

/** «1 партия», «2 партии», «9 партий». */
export const batchesCount = (count: number) =>
	countOf(count, 'партия', 'партии', 'партий');

export function statusLabel(status: Delivery['status']): string {
	return status === 'approved' ? 'принята' : 'ожидает приёмки';
}

export function itemName(item: DeliveryItem): string {
	return item.product_name ?? 'Продукт не найден в каталоге';
}

/** Единица позиции; без продукта в каталоге количество показывается как есть. */
export function itemUnit(item: DeliveryItem): QuantityUnit {
	return item.unit ?? 'pcs';
}
