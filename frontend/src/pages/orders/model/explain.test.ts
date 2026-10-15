import { describe, expect, it } from 'vitest';

import type { HotDish, HotReason } from '@entities/dish';
import { withUnits } from '@shared/lib/test-utils';

import { explainReason } from './explain';

function reason(overrides: Partial<HotReason> = {}): HotReason {
	return {
		product: {
			id: 'cream',
			company_id: 'demo',
			archived: false,
			name: 'Сливки 33%',
			unit: 'ml',
			category: 'Молочные',
		},
		at_risk_quantity: 500,
		batch_id: 'batch-1',
		expires_at: '2026-10-14T20:40:00Z',
		hours_left: 6,
		required_quantity: 80,
		dishes_using: 2,
		urgency: 0.9,
		coverage: 0.8,
		significance: 1,
		exclusivity: 0.7,
		...overrides,
	};
}

function dish(suggested: number, reasons = [reason()]): HotDish {
	return {
		dish: { id: 'carbonara', name: 'Паста карбонара' },
		priority: 120,
		available_portions: 15,
		suggested_portions: suggested,
		reasons,
	};
}

describe('explainReason', () => {
	it('часть под риском: сколько уйдёт из скольких и сколько ещё блюд с продуктом', () => {
		expect(explainReason(dish(5), reason())).toMatch(
			withUnits(
				'На порцию 80 мл: 5 порций израсходуют 400 мл из 500 мл · используется ещё в 1 блюде'
			)
		);
	});

	it('порций хватает на всё под риском, продукт только в этом блюде', () => {
		const bacon = reason({
			product: { ...reason().product, name: 'Бекон', unit: 'g' },
			at_risk_quantity: 300,
			required_quantity: 60,
			dishes_using: 1,
		});

		expect(explainReason(dish(5, [bacon]), bacon)).toMatch(
			withUnits(
				'На порцию 60 г: 5 порций израсходуют всё · используется только в этом блюде'
			)
		);
	});

	it('под риском меньше порции: блюдо ниже в списке', () => {
		const sourCream = reason({
			product: { ...reason().product, name: 'Сметана 20%', unit: 'g' },
			at_risk_quantity: 20,
			required_quantity: 30,
			significance: 0.67,
			dishes_using: 4,
		});

		expect(explainReason(dish(1, [sourCream]), sourCream)).toMatch(
			withUnits(
				'Меньше порции: на порцию нужно 30 г, поэтому блюдо ниже в списке · используется ещё в 3 блюдах'
			)
		);
	});
});
