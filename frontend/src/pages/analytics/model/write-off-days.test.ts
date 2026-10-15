import { describe, expect, it } from 'vitest';

import { testProduct } from '@entities/product/index.testing';

import type { WriteOffDay } from '../api/analytics';
import { dayBars, writtenOffProducts } from './write-off-days';

const product = (name: string) => testProduct({ id: name, name });

const tomatoes = product('Томаты');
const bacon = product('Бекон');

const items: WriteOffDay[] = [
	{
		day: '2026-10-12',
		product: tomatoes,
		reason: 'expired',
		quantity: 300,
		operations: 2,
	},
	{
		day: '2026-10-12',
		product: bacon,
		reason: 'physical_damage',
		quantity: 100,
		operations: 1,
	},
	{
		day: '2026-10-14',
		product: tomatoes,
		reason: 'transport_damage',
		quantity: 500,
		operations: 1,
	},
];

const period = {
	choice: 'custom' as const,
	start: new Date(2026, 9, 11),
	end: new Date(2026, 9, 15),
};
const now = new Date(2026, 9, 14, 18, 0);

describe('списания по дням', () => {
	it('операции по дням периода, пустой день - нули', () => {
		const bars = dayBars(items, period, now);

		expect(bars.days).toHaveLength(4);
		expect(bars.expired).toEqual([0, 2, 0, 0]);
		expect(bars.other).toEqual([0, 1, 0, 1]);
	});

	it('с выбранным продуктом - его количество', () => {
		const bars = dayBars(items, period, now, tomatoes.id);

		expect(bars.expired).toEqual([0, 300, 0, 0]);
		expect(bars.other).toEqual([0, 0, 0, 500]);
	});

	it('продукты для выбора - по названию, без повторов', () => {
		expect(writtenOffProducts(items).map((item) => item.name)).toEqual([
			'Бекон',
			'Томаты',
		]);
	});
});
