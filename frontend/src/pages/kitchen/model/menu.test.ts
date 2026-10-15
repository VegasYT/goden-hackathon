import { describe, expect, it } from 'vitest';

import type { DishAvailability, PointDish } from '@entities/dish';

import { disabledDishes, visibleMenu } from './menu';

function dish(
	name: string,
	portions: number,
	products: string[] = []
): DishAvailability {
	return {
		dish: { id: name, name },
		available: portions > 0,
		portions,
		limiting: [],
		ingredients: products.map((product) => ({
			product: {
				id: product,
				company_id: 'demo',
				archived: false,
				name: product,
				unit: 'g',
				category: '',
			},
			required_quantity: 100,
			available_quantity: 100 * portions,
			expired_quantity: 0,
		})),
	};
}

const menu = [
	dish('Паста карбонара', 15, ['Спагетти', 'Бекон']),
	dish('Брускетта с томатами', 0, ['Томаты']),
	dish('Омлет', 8, ['Яйцо куриное']),
	dish('Чизкейк Нью-Йорк', 0),
];

const names = (items: DishAvailability[]) =>
	items.map((item) => item.dish.name);

describe('меню кухни', () => {
	it('ищет по продукту рецептуры без учёта регистра', () => {
		expect(names(visibleMenu(menu, 'БЕКОН', 'name'))).toEqual([
			'Паста карбонара',
		]);
		expect(names(visibleMenu(menu, ' омл ', 'name'))).toEqual(['Омлет']);
	});

	it('по порциям ставит стоп-лист первым, равные — по названию', () => {
		expect(names(visibleMenu(menu, '', 'portions'))).toEqual([
			'Брускетта с томатами',
			'Чизкейк Нью-Йорк',
			'Омлет',
			'Паста карбонара',
		]);
	});

	it('выключенные отбираются по названию и идут по алфавиту', () => {
		const dishes: PointDish[] = [
			{ dish_id: '1', name: 'Солянка сборная', enabled: false },
			{ dish_id: '2', name: 'Паста карбонара', enabled: true },
			{ dish_id: '3', name: 'Окрошка', enabled: false },
		];

		expect(disabledDishes(dishes, '').map((item) => item.name)).toEqual([
			'Окрошка',
			'Солянка сборная',
		]);
		expect(disabledDishes(dishes, 'соля').map((item) => item.name)).toEqual([
			'Солянка сборная',
		]);
	});
});
