import { describe, expect, it } from 'vitest';

import type { DeliveryItem } from '@entities/delivery';
import { testDeliveryItem } from '@entities/delivery/index.testing';

import {
	correctionBody,
	correctionProblems,
	initialCorrection,
	isChanged,
	type CorrectionValue,
	type ServerHint,
} from './correction';

const now = new Date('2026-10-12T12:00:00Z');

function item(overrides: Partial<DeliveryItem> = {}): DeliveryItem {
	return testDeliveryItem({
		id: 'salmon',
		product_id: 'product-salmon',
		product_name: 'Лосось охлаждённый',
		unit: 'g',
		declared_quantity: 12000,
		expires_at: '2026-10-14T09:00:00Z',
		accepted_quantity: 12000,
		batch_ids: ['b1'],
		in_stock_quantity: 6500,
		...overrides,
	});
}

const value = (
	accepted: number | null,
	rejected: number | null,
	quarantined: number | null = 0
): CorrectionValue => ({ accepted, rejected, quarantined });

describe('correctionProblems', () => {
	it.each<[string, CorrectionValue, ServerHint, string[]]>([
		['без изменений', value(12000, 0), {}, []],
		['уменьшение в пределах остатка', value(5500, 6500), {}, []],
		['уменьшение сверх остатка', value(4000, 8000), {}, ['stock-consumed']],
		[
			'предел сервера меньше остатка',
			value(5500, 6500),
			{ limit: 3000 },
			['stock-consumed'],
		],
		['сумма меньше заявленного', value(10000, null), {}, ['unallocated']],
		['сумма больше заявленного', value(12000, 1000), {}, ['over']],
	])('%s', (_, next, hint, problems) => {
		expect(correctionProblems(item(), next, now, hint)).toEqual(problems);
	});

	it('у просроченной позиции принятое можно уменьшить, но не увеличить', () => {
		const tomatoes = item({
			declared_quantity: 2500,
			accepted_quantity: 2000,
			rejected_quantity: 500,
			in_stock_quantity: 2000,
			expires_at: '2026-10-11T18:00:00Z',
		});

		expect(correctionProblems(tomatoes, value(1500, 1000), now)).toEqual([]);
		expect(correctionProblems(tomatoes, value(2500, 0), now)).toEqual([
			'accepted-expired',
		]);
	});
});

describe('тело исправления', () => {
	it('уходят только изменённые позиции и причина без пробелов по краям', () => {
		const salmon = item();
		const flour = item({
			id: 'flour',
			declared_quantity: 25000,
			accepted_quantity: 25000,
			in_stock_quantity: 25000,
		});

		expect(isChanged(salmon, initialCorrection(salmon))).toBe(false);
		expect(
			correctionBody(
				[salmon, flour],
				[initialCorrection(salmon), value(20000, 5000)],
				' Пересчитали после разгрузки '
			)
		).toEqual({
			reason: 'Пересчитали после разгрузки',
			items: [
				{
					item_id: 'flour',
					accepted_quantity: 20000,
					rejected_quantity: 5000,
					quarantined_quantity: 0,
				},
			],
		});
	});
});
