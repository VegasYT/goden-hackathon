import { describe, expect, it } from 'vitest';

import type { DeliveryRecommendation } from '../api/recommendations';
import { coverLabel } from './reorder';

const recommendation = (
	overrides: Partial<DeliveryRecommendation>
): DeliveryRecommendation => ({
	product: {
		id: 'arugula',
		company_id: 'demo',
		archived: false,
		name: 'Руккола',
		unit: 'g',
		category: 'Овощи и зелень',
		shelf_life_days: 3,
	},
	available_quantity: 150,
	consumed_quantity: 400,
	cover_hours: 9.4,
	suggested_quantity: 1050,
	reason: 'low_cover',
	needed_by: [],
	...overrides,
});

describe('на сколько хватит', () => {
	it.each([
		['часы вниз', { cover_hours: 9.9 }, '9 ч'],
		['меньше часа', { cover_hours: 0.4 }, 'меньше часа'],
		['не было расхода', { cover_hours: undefined, consumed_quantity: 0 }, null],
		['продукта нет', { available_quantity: 0, cover_hours: 0 }, null],
	] as const)('%s', (_, overrides, label) => {
		expect(coverLabel(recommendation(overrides))).toBe(label);
	});
});
