import {
	menuMatcher,
	queryMatcher,
	type DishAvailability,
	type PointDish,
} from '@entities/dish';

import type { KitchenSort } from './search';

const collator = new Intl.Collator('ru');

/** Блюда меню по запросу и в выбранном порядке: при `portions` стоп-лист первым, равные по названию. */
export function visibleMenu(
	menu: DishAvailability[],
	query: string,
	sort: KitchenSort
): DishAvailability[] {
	return menu
		.filter(menuMatcher(query, { byIngredients: true }))
		.toSorted(
			(a, b) =>
				(sort === 'portions' ? a.portions - b.portions : 0) ||
				collator.compare(a.dish.name, b.dish.name)
		);
}

/** Выключенные на точке блюда по запросу в названии, по названию. */
export function disabledDishes(
	dishes: PointDish[],
	query: string
): PointDish[] {
	const matches = queryMatcher(query);

	return dishes
		.filter((dish) => !dish.enabled && matches(dish.name))
		.toSorted((a, b) => collator.compare(a.name, b.name));
}
