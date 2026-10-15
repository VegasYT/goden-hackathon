import type {
	DishAvailability,
	HotDish,
	HotOfferSetting,
} from '@entities/dish';
import type { components } from '@shared/api';

import { mockGet } from './test-api';
import { carbonara, testMenu } from './test-kitchen';
import { testProduct } from './test-stock';

type Order = components['schemas']['Order'];

export const cream = testProduct({
	id: 'product-cream',
	name: 'Сливки 33%',
	unit: 'ml',
});

/** Официант, которая оформляет заказы по умолчанию. */
export const olgaActor = {
	type: 'user' as const,
	id: 'olga',
	name: 'Ольга Смирнова',
};

/** Сервисная учётная запись Мигома. */
export const migomActor = { type: 'service' as const, id: 'migom-integration' };

export function testOrder(overrides: Partial<Order> = {}): Order {
	return {
		id: 'order-1',
		point_id: 'lenina',
		status: 'accepted',
		placed_at: '2026-10-14T11:33:00Z',
		placed_by: olgaActor,
		items: [{ dish_id: 'dish-Омлет', dish_name: 'Омлет', portions: 1 }],
		shortages: [],
		...overrides,
	};
}

/** Отклонённый заказ: сливок нужно 160 мл, было 90 мл. */
export const rejectedOrder = testOrder({
	id: 'order-rejected',
	status: 'rejected',
	placed_at: '2026-10-14T11:31:00Z',
	items: [
		{ dish_id: carbonara.dish.id, dish_name: 'Паста карбонара', portions: 2 },
		{ dish_id: 'dish-Омлет', dish_name: 'Омлет', portions: 1 },
	],
	shortages: [
		{
			product_id: cream.id,
			product: cream,
			required_quantity: 160,
			available_quantity: 90,
		},
	],
});

/** Горячее предложение карбонары: под риском 500 мл сливок, партия истекает через 6 часов. */
export function hotCarbonara(now = new Date()): HotDish {
	return {
		dish: carbonara.dish,
		priority: 120,
		available_portions: 15,
		suggested_portions: 5,
		reasons: [
			{
				product: cream,
				at_risk_quantity: 500,
				batch_id: 'batch-cream',
				expires_at: new Date(now.getTime() + 6.5 * 3_600_000).toISOString(),
				hours_left: 6,
				required_quantity: 80,
				dishes_using: 2,
				urgency: 0.9,
				coverage: 0.8,
				significance: 1,
				exclusivity: 0.7,
			},
		],
	};
}

interface OrdersApiState {
	orders?: Order[] | ((params: Record<string, unknown>) => Order[]);
	total?: number;
	menu?: DishAvailability[];
	hot?: HotDish[];
	/** Ручные настройки горячих предложений; по умолчанию их нет. */
	settings?: HotOfferSetting[];
}

/** Ответы раздела заказов: история, меню и горячие предложения; состояние можно менять между запросами. */
export function ordersApi(state: OrdersApiState = {}) {
	return mockGet((url, params) => {
		if (url.endsWith('/orders')) {
			const orders =
				typeof state.orders === 'function'
					? state.orders(params)
					: (state.orders ?? [testOrder(), rejectedOrder]);

			return { items: orders, total: state.total ?? orders.length };
		}

		if (url.endsWith('/recommendations/dishes/settings')) {
			return { items: state.settings ?? [] };
		}

		if (url.endsWith('/recommendations/dishes')) {
			const hot = state.hot ?? [];

			return { items: hot, total: hot.length };
		}

		if (url.endsWith('/menu')) {
			const menu = state.menu ?? testMenu;

			return { items: menu, total: menu.length };
		}

		throw new Error(`Неожиданный запрос ${url}`);
	});
}

/** Сотрудник, который закрепил горячее предложение. */
export const ilyaActor = {
	type: 'user' as const,
	id: 'ilya',
	name: 'Илья Смирнов',
};

/** Горячее предложение, закреплённое управляющим, по умолчанию без продуктов под риском. */
export function pinnedHot(
	base: HotDish,
	comment: string,
	setAt = new Date()
): HotDish {
	return {
		...base,
		reasons: [],
		manual: { comment, set_by: ilyaActor, set_at: setAt.toISOString() },
	};
}
