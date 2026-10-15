import type { Delivery, DeliveryItem } from './api/delivery-queries';

/** Позиция поставки для тестов: 4 кг пармезана, ещё не принятая. */
export function testDeliveryItem(
	overrides: Partial<DeliveryItem> = {}
): DeliveryItem {
	return {
		id: 'parmesan',
		product_id: 'product-parmesan',
		product_name: 'Пармезан 24 мес.',
		unit: 'g',
		product_category: 'Молочные продукты',
		declared_quantity: 4000,
		expires_at: '2099-12-20T23:59:00Z',
		accepted_quantity: 0,
		rejected_quantity: 0,
		quarantined_quantity: 0,
		expired: false,
		batch_ids: [],
		quarantine_resolutions: [],
		in_stock_quantity: 0,
		...overrides,
	};
}

/** Ожидающая приёмки поставка № 118 точки `lenina` с одной позицией. */
export function testDelivery(overrides: Partial<Delivery> = {}): Delivery {
	return {
		id: 'delivery-118',
		point_id: 'lenina',
		number: '118',
		delivered_at: '2026-10-10T09:40:00Z',
		status: 'registered',
		items: [testDeliveryItem()],
		...overrides,
	};
}
