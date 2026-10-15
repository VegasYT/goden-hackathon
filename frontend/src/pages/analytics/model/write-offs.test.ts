import { describe, expect, it } from 'vitest';

import type { Product } from '@entities/product';
import { testProduct } from '@entities/product/index.testing';

import type { WriteOffStatistic } from '../api/analytics';
import { writeOffsByProduct, writeOffsByReason } from './write-offs';

const product = (name: string, unit: Product['unit']) =>
	testProduct({ id: name, name, unit });

const tomatoes = product('Томаты', 'g');
const cheese = product('Сыр пармезан', 'g');
const milk = product('Молоко', 'ml');
const eggs = product('Яйцо', 'pcs');

const items: WriteOffStatistic[] = [
	{ product: eggs, reason: 'expired', quantity: 30, operations: 3 },
	{ product: milk, reason: 'storage_violation', quantity: 5000, operations: 1 },
	{ product: tomatoes, reason: 'expired', quantity: 2100, operations: 2 },
	{ product: cheese, reason: 'expired', quantity: 73500, operations: 51 },
	{
		product: tomatoes,
		reason: 'transport_damage',
		quantity: 1800,
		operations: 1,
	},
];

describe('свёртки списаний', () => {
	it('продукт с двумя причинами - одна строка с суммой', () => {
		const tomatoesRow = writeOffsByProduct(items).find(
			(row) => row.product === tomatoes
		);

		expect(tomatoesRow).toEqual({
			product: tomatoes,
			quantity: 3900,
			operations: 3,
			reasons: ['expired', 'transport_damage'],
		});
	});

	it('продукты идут по единице, внутри неё по убыванию количества', () => {
		expect(writeOffsByProduct(items).map((row) => row.product.name)).toEqual([
			'Сыр пармезан',
			'Томаты',
			'Молоко',
			'Яйцо',
		]);
	});

	it('причины - по числу операций, с продуктами в порядке списка по продуктам', () => {
		const reasons = writeOffsByReason(items);

		expect(reasons.map((row) => [row.reason, row.operations])).toEqual([
			['expired', 56],
			['storage_violation', 1],
			['transport_damage', 1],
		]);
		expect(reasons[0].products.map((row) => row.product.name)).toEqual([
			'Сыр пармезан',
			'Томаты',
			'Яйцо',
		]);
	});
});
