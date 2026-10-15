import { describe, expect, it } from 'vitest';

import type { MenuIngredient } from '../api/dish-queries';
import { portionsOf, shortageOf } from './dish';

function ingredient(
	required: number,
	available: number,
	name = 'Сливки 33%'
): MenuIngredient {
	return {
		product: {
			id: name,
			company_id: 'demo',
			archived: false,
			name,
			unit: 'ml',
			category: 'Молочные продукты',
		},
		required_quantity: required,
		available_quantity: available,
		expired_quantity: 0,
	};
}

describe('продукт рецептуры', () => {
	it('порций хватит с округлением вниз', () => {
		expect(portionsOf(ingredient(80, 1200))).toBe(15);
		expect(portionsOf(ingredient(60, 1300))).toBe(21);
		expect(portionsOf(ingredient(100, 60))).toBe(0);
	});

	it('нехватка на порцию нулевая, если продукта хватает', () => {
		expect(shortageOf(ingredient(100, 60))).toBe(40);
		expect(shortageOf(ingredient(80, 1200))).toBe(0);
	});
});
