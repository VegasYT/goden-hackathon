import { describe, expect, it } from 'vitest';

import type { Dish } from '@entities/dish';

import { quantityProblems, recipeLine, sameRecipe, recipeRows } from './recipe';

const nbsp = '\u00a0';

const omelette: Dish = {
	id: 'omelette',
	company_id: 'demo',
	name: 'Омлет',
	archived: false,
	ingredients: [
		{
			product_id: 'eggs',
			product_name: 'Яйцо куриное',
			unit: 'pcs',
			quantity: 3,
			product_archived: false,
		},
		{
			product_id: 'milk',
			product_name: 'Молоко',
			unit: 'ml',
			quantity: 50,
			product_archived: false,
		},
	],
};

describe('рецептура', () => {
	it('строкой в единицах продукта', () => {
		expect(recipeLine(omelette)).toBe(
			`Яйцо куриное 3${nbsp}шт · Молоко 50${nbsp}мл`
		);
	});

	it('пустое и нулевое количество — ошибки, целое положительное — нет', () => {
		const rows = recipeRows(omelette);

		expect(
			quantityProblems([
				{ ...rows[0]!, quantity: null },
				{ ...rows[1]!, quantity: 0 },
			])
		).toEqual({
			eggs: 'Введите количество больше нуля',
			milk: 'Введите количество больше нуля',
		});
		expect(quantityProblems(rows)).toEqual({});
	});

	it('черновик без изменений совпадает с блюдом, другое количество — нет', () => {
		const rows = recipeRows(omelette);

		expect(sameRecipe(rows, omelette)).toBe(true);
		expect(
			sameRecipe([rows[0]!, { ...rows[1]!, quantity: 60 }], omelette)
		).toBe(false);
	});
});
