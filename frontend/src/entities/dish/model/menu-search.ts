import type { DishAvailability } from '../api/dish-queries';

/** Проверка текста по запросу поиска без учёта регистра; пустой запрос подходит любому тексту. */
export function queryMatcher(query: string): (text: string) => boolean {
	const needle = query.trim().toLocaleLowerCase('ru');

	return (text) => text.toLocaleLowerCase('ru').includes(needle);
}

/** Поиск по меню точки: по названию блюда, с `byIngredients` ещё и по продуктам рецептуры. */
export function menuMatcher(
	query: string,
	{ byIngredients }: { byIngredients: boolean }
): (item: DishAvailability) => boolean {
	const matches = queryMatcher(query);

	return (item) =>
		matches(item.dish.name) ||
		(byIngredients &&
			item.ingredients.some((ingredient) => matches(ingredient.product.name)));
}
