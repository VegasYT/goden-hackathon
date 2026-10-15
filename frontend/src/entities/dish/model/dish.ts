import { countOf } from '@shared/lib/plural';
import { formatAmount } from '@shared/lib/quantity';

import type { DishAvailability, MenuIngredient } from '../api/dish-queries';

/** На сколько порций хватит доступного остатка продукта. */
export function portionsOf(
	ingredient: Pick<MenuIngredient, 'available_quantity' | 'required_quantity'>
): number {
	return Math.floor(
		ingredient.available_quantity / ingredient.required_quantity
	);
}

/** Сколько продукта не хватает на одну порцию; ноль, если хватает. */
export function shortageOf(ingredient: MenuIngredient): number {
	return Math.max(
		0,
		ingredient.required_quantity - ingredient.available_quantity
	);
}

/** Количество продукта рецептуры в его единице, как на складе: «60 г», «1,2 л». */
export function ingredientQuantity(
	ingredient: MenuIngredient,
	value: number
): string {
	return formatAmount(value, ingredient.product.unit);
}

/** Блюда меню, которые сейчас нельзя приготовить. */
export function stoppedDishes(menu: DishAvailability[]): DishAvailability[] {
	return menu.filter((dish) => !dish.available);
}

export function stopReasonLabel(dish: DishAvailability): string {
	return dish.reason === 'no_recipe' ? 'нет рецептуры' : 'не хватает продуктов';
}

/** Почему блюдо нельзя приготовить: ограничивающий продукт с остатком и количеством на порцию. */
export function stopCaption(dish: DishAvailability): string {
	if (dish.reason === 'no_recipe') {
		return 'Нет рецептуры';
	}

	const [first, ...rest] = dish.limiting;

	if (!first) {
		return 'Не хватает продуктов';
	}

	return rest.length > 0
		? `Не хватает: ${dish.limiting.map((item) => item.product.name).join(', ')}`
		: `Не хватает: ${first.product.name} · есть ${ingredientQuantity(first, first.available_quantity)}, на порцию ${ingredientQuantity(first, first.required_quantity)}`;
}

/** «1 порция», «5 порций». */
export const portionsText = (count: number) =>
	countOf(count, 'порция', 'порции', 'порций');
