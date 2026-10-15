import { describe, expect, it } from 'vitest';

import type { DishAvailability } from '@entities/dish';

import { droppedDishes, orderBody, orderLines, overLimit } from './order-form';

function dish(id: string, portions: number): DishAvailability {
	return {
		dish: { id, name: id },
		available: portions > 0,
		portions,
		reason: portions > 0 ? undefined : 'insufficient_stock',
		limiting: [],
		ingredients: [],
	};
}

const menu = [dish('borscht', 18), dish('bruschetta', 0), dish('caesar', 1)];

describe('заказ по меню', () => {
	it('тело заказа — только блюда меню с порциями, в порядке меню', () => {
		expect(orderBody(menu, { caesar: 1, borscht: 2, gone: 3 })).toEqual({
			items: [
				{ dish_id: 'borscht', portions: 2 },
				{ dish_id: 'caesar', portions: 1 },
			],
		});
	});

	it('пустой заказ тела не даёт', () => {
		expect(orderBody(menu, { borscht: 0 })).toBeUndefined();
	});

	it('после уменьшения порций в меню превышение отмечено и заказ не отправляется', () => {
		const fresher = [
			dish('borscht', 1),
			dish('bruschetta', 0),
			dish('caesar', 1),
		];
		const portions = { borscht: 2, caesar: 1 };

		expect([...overLimit(fresher, portions)]).toEqual(['borscht']);
		expect(orderBody(fresher, portions)).toBeUndefined();
	});

	it('блюдо пропало из меню: оно названо, остальные строки на месте', () => {
		const fresher = [dish('borscht', 18), dish('bruschetta', 0)];
		const portions = { borscht: 2, caesar: 1 };

		const sent = orderLines(menu, portions).map((line) => line.dish);

		expect(droppedDishes(sent, fresher)).toEqual([
			{ id: 'caesar', name: 'caesar' },
		]);
		expect(orderLines(fresher, portions).map((line) => line.dish.id)).toEqual([
			'borscht',
		]);
	});
});
