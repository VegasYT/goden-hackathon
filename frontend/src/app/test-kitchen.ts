import {
	portionsOf,
	type DishAvailability,
	type MenuIngredient,
	type PointDish,
} from '@entities/dish';
import type { QuantityUnit } from '@shared/lib/quantity';

import { mockGet } from './test-api';

/** Продукт рецептуры: количество на порцию, доступный и просроченный остаток в единице продукта. */
export function testIngredient(
	name: string,
	unit: QuantityUnit,
	required: number,
	available: number,
	expired = 0
): MenuIngredient {
	return {
		product: {
			id: `product-${name}`,
			company_id: 'demo',
			archived: false,
			name,
			unit,
			category: '',
		},
		required_quantity: required,
		available_quantity: available,
		expired_quantity: expired,
	};
}

/** Блюдо меню; порции, причину и ограничивающие продукты считает тем же правилом, что сервер. */
export function testDish(
	name: string,
	ingredients: MenuIngredient[]
): DishAvailability {
	const portions = ingredients.length
		? Math.min(...ingredients.map(portionsOf))
		: 0;

	return {
		dish: { id: `dish-${name}`, name },
		available: portions > 0,
		portions,
		reason:
			portions > 0
				? undefined
				: ingredients.length
					? 'insufficient_stock'
					: 'no_recipe',
		limiting: ingredients.filter(
			(ingredient) => portionsOf(ingredient) === portions
		),
		ingredients,
	};
}

export const carbonara = testDish('Паста карбонара', [
	testIngredient('Спагетти', 'g', 120, 4800),
	testIngredient('Бекон', 'g', 60, 1300, 200),
	testIngredient('Сливки 33%', 'ml', 80, 1200),
]);

export const bruschetta = testDish('Брускетта с томатами', [
	testIngredient('Хлеб пшеничный', 'g', 80, 1600),
	testIngredient('Томаты', 'g', 100, 60),
]);

export const chicken = testDish('Куриное филе в сливочном соусе', [
	testIngredient('Куриное филе', 'g', 180, 150, 900),
	testIngredient('Сливки 33%', 'ml', 60, 1200),
]);

export const omelet = testDish('Омлет', [
	testIngredient('Яйцо куриное', 'pcs', 3, 24),
	testIngredient('Молоко', 'ml', 50, 3000),
]);

export const cheesecake = testDish('Чизкейк Нью-Йорк', []);

export const testMenu = [carbonara, bruschetta, chicken, omelet, cheesecake];

/** Блюда компании на точке: блюда меню включены, `disabled` выключены. */
export function pointDishes(
	menu: DishAvailability[],
	disabled: string[] = ['Солянка сборная']
): PointDish[] {
	return [
		...menu.map(({ dish }) => ({
			dish_id: dish.id,
			name: dish.name,
			enabled: true,
		})),
		...disabled.map((name) => ({
			dish_id: `dish-${name}`,
			name,
			enabled: false,
		})),
	];
}

/** Ответы меню и блюд точки; состояние можно менять между запросами, ответ строится при каждом. */
export function kitchenApi(state: {
	menu: DishAvailability[];
	dishes?: PointDish[];
}) {
	return mockGet((url) => {
		if (url.endsWith('/menu')) {
			return { items: state.menu, total: state.menu.length };
		}

		if (url.endsWith('/dishes')) {
			const dishes = state.dishes ?? pointDishes(state.menu);

			return { items: dishes, total: dishes.length };
		}

		throw new Error(`Неожиданный запрос ${url}`);
	});
}
