import type { DishAvailability, DishRef } from '@entities/dish';

import type { NewOrder } from '../api/orders';

/** Порции заказа по идентификатору блюда; блюда без порций в заказ не входят. */
export type Portions = Readonly<Record<string, number>>;

export interface OrderLine {
	dish: DishRef;
	portions: number;
}

/** Сколько порций блюда можно заказать: меню совпадает с решением заказа, у блюда стоп-листа — ноль. */
export const limitOf = (dish: DishAvailability) =>
	dish.available ? dish.portions : 0;

/** Строки заказа в порядке меню; блюда, которых нет в меню, не входят. */
export function orderLines(
	menu: DishAvailability[],
	portions: Portions
): OrderLine[] {
	return menu
		.filter((item) => (portions[item.dish.id] ?? 0) > 0)
		.map((item) => ({ dish: item.dish, portions: portions[item.dish.id] }));
}

/** Блюда, порций которых больше, чем можно приготовить: например, после обновления меню. */
export function overLimit(
	menu: DishAvailability[],
	portions: Portions
): ReadonlySet<string> {
	return new Set(
		menu
			.filter((dish) => (portions[dish.dish.id] ?? 0) > limitOf(dish))
			.map((dish) => dish.dish.id)
	);
}

/** Блюда отправленного заказа, которых нет в новом меню: их выключили на точке. */
export function droppedDishes(
	sent: DishRef[],
	current: DishAvailability[]
): DishRef[] {
	const now = new Set(current.map((item) => item.dish.id));

	return sent.filter((dish) => !now.has(dish.id));
}

/** Порции без перечисленных блюд. */
export function withoutDishes(portions: Portions, dishIds: string[]): Portions {
	return Object.fromEntries(
		Object.entries(portions).filter(([dishId]) => !dishIds.includes(dishId))
	);
}

/** Тело заказа или `undefined`, если заказывать нечего или порций больше, чем можно приготовить. */
export function orderBody(
	menu: DishAvailability[],
	portions: Portions
): NewOrder | undefined {
	const lines = orderLines(menu, portions);

	if (lines.length === 0 || overLimit(menu, portions).size > 0) {
		return undefined;
	}

	return {
		items: lines.map((line) => ({
			dish_id: line.dish.id,
			portions: line.portions,
		})),
	};
}
