import type {
	Delivery,
	DeliveryPage,
	DeliverySummary,
} from '@entities/delivery';
import { testDelivery } from '@entities/delivery/index.testing';
import { ApiError } from '@shared/api';

import { mockGet } from './test-api';

export {
	testDelivery,
	testDeliveryItem,
} from '@entities/delivery/index.testing';

export function testSummary(
	overrides: Partial<DeliverySummary> = {}
): DeliverySummary {
	return {
		total: 1,
		registered: 1,
		approved: 0,
		open_quarantine_items: 0,
		...overrides,
	};
}

/** Та же поставка, принятая целиком: по умолчанию её приняла Анна Котова в 14:39. */
export function approvedDelivery(
	delivery: Delivery,
	approvedBy: Delivery['approved_by'] = {
		type: 'user',
		id: 'anna',
		name: 'Анна Котова',
	}
): Delivery {
	return {
		...delivery,
		status: 'approved',
		approved_at: new Date(2026, 9, 12, 14, 39).toISOString(),
		approved_by: approvedBy,
		items: delivery.items.map((item) => ({
			...item,
			accepted_quantity: item.declared_quantity,
			batch_ids: ['batch'],
		})),
	};
}

/** Ответы списка, сводки и карточки поставок точки `lenina`. */
export function deliveriesApi({
	list = [testDelivery()],
	summary = testSummary(),
}: {
	list?: Delivery[] | ((params: Record<string, unknown>) => DeliveryPage);
	summary?: DeliverySummary;
} = {}) {
	return mockGet((url, params) => {
		if (url.endsWith('/deliveries/summary')) {
			return summary;
		}

		if (url.endsWith('/deliveries')) {
			return typeof list === 'function'
				? list(params)
				: { items: list, total: list.length };
		}

		const found = Array.isArray(list)
			? list.find((delivery) => url.endsWith(`/deliveries/${delivery.id}`))
			: undefined;

		if (!found) {
			throw new ApiError('Поставка не найдена', { status: 404 });
		}

		return found;
	});
}
