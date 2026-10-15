import type { Dish, Ingredient } from '@entities/dish';
import type { Product } from '@entities/product';
import { formatPortion, type QuantityUnit } from '@shared/lib/quantity';

/** Строка черновика рецептуры: количество пустое, пока его не ввели. */
export interface RecipeRow {
	productId: string;
	name: string;
	unit: QuantityUnit;
	quantity: number | null;
	archived: boolean;
}

export function recipeRows(dish: Dish): RecipeRow[] {
	return dish.ingredients.map((ingredient) => ({
		productId: ingredient.product_id,
		name: ingredient.product_name,
		unit: ingredient.unit,
		quantity: ingredient.quantity,
		archived: ingredient.product_archived,
	}));
}

export function rowOf(product: Product): RecipeRow {
	return {
		productId: product.id,
		name: product.name,
		unit: product.unit,
		quantity: null,
		archived: false,
	};
}

/** Рецептура одной строкой: «Яйцо куриное 3 шт · Молоко 50 мл». */
export function recipeLine(dish: Dish): string {
	return dish.ingredients
		.map(
			(ingredient) =>
				`${ingredient.product_name} ${formatPortion(ingredient.quantity, ingredient.unit)}`
		)
		.join(' · ');
}

/** Продукты рецептуры в архиве: из-за них блюдо нельзя восстановить. */
export const archivedProducts = (dish: Dish) =>
	dish.ingredients
		.filter((ingredient) => ingredient.product_archived)
		.map((ingredient) => ingredient.product_name);

/** Ошибки количеств по продукту: пустое или не больше нуля. */
export function quantityProblems(rows: RecipeRow[]): Record<string, string> {
	return Object.fromEntries(
		rows
			.filter(
				(row) =>
					row.quantity === null ||
					!Number.isInteger(row.quantity) ||
					row.quantity <= 0
			)
			.map((row) => [row.productId, 'Введите количество больше нуля'])
	);
}

export function toIngredients(rows: RecipeRow[]): Ingredient[] {
	return rows.map((row) => ({
		product_id: row.productId,
		quantity: row.quantity ?? 0,
	}));
}

/** Черновик совпадает с рецептурой блюда: те же продукты в том же порядке и количествах. */
export function sameRecipe(rows: RecipeRow[], dish: Dish): boolean {
	return (
		rows.length === dish.ingredients.length &&
		rows.every(
			(row, index) =>
				row.productId === dish.ingredients[index]?.product_id &&
				row.quantity === dish.ingredients[index]?.quantity
		)
	);
}
