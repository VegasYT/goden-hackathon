import { describe, expect, it } from 'vitest';

import type { Batch } from '../api/stock';
import {
	adjustProblems,
	firstInLine,
	productStock,
	toAdjustment,
	toWriteOff,
	usableBatches,
	writeOffProblems,
	type WriteOffValues,
} from './actions';

const now = new Date(2026, 9, 12, 14, 32);

function batch(overrides: Partial<Batch> = {}): Batch {
	return {
		id: 'b-111',
		point_id: 'lenina',
		product_id: 'tomato',
		delivery_id: 'd-111',
		delivery_number: '111',
		item_id: 'i-111',
		received_quantity: 2000,
		quantity: 1600,
		expires_at: new Date(2026, 9, 13, 8, 0).toISOString(),
		received_at: new Date(2026, 9, 8, 15, 51).toISOString(),
		...overrides,
	};
}

const expired = batch({
	id: 'b-109',
	expires_at: new Date(2026, 9, 12, 14, 30).toISOString(),
});

const valid: WriteOffValues = {
	batchId: 'b-111',
	quantity: 500,
	reason: 'physical_damage',
	comment: ' Порвался ящик ',
};

describe('остаток продукта по партиям', () => {
	it('просроченная партия не входит в доступное, но считается партией с остатком', () => {
		expect(
			productStock([expired, batch(), batch({ id: 'b-113', quantity: 0 })], now)
		).toEqual({ available: 1600, batches: 2 });
	});
});

describe('списание', () => {
	it.each([
		['верное', valid, {}],
		[
			'без количества',
			{ ...valid, quantity: null },
			{ quantity: 'Укажите, сколько списать' },
		],
		[
			'больше остатка',
			{ ...valid, quantity: 2000 },
			{
				quantity: 'В партии осталось 1,6\u00a0кг — меньше, чем вы списываете',
			},
		],
		['без причины', { ...valid, reason: null }, { reason: 'Выберите причину' }],
		[
			'просрочка у годной партии',
			{ ...valid, reason: 'expired' as const },
			{ reason: 'Срок годности партии ещё не истёк' },
		],
		[
			'без комментария',
			{ ...valid, comment: '  ' },
			{ comment: 'Напишите, что случилось с продуктом' },
		],
	])('%s', (_, values, errors) => {
		expect(writeOffProblems(values, batch(), 'g', now)).toEqual(errors);
	});

	it('просрочку можно указать у истёкшей партии, комментарий для неё необязателен', () => {
		expect(
			writeOffProblems(
				{ ...valid, reason: 'expired', comment: '' },
				expired,
				'g',
				now
			)
		).toEqual({});
	});

	it('тело запроса с обрезанным комментарием', () => {
		expect(toWriteOff(valid)).toEqual({
			quantity: 500,
			reason: 'physical_damage',
			comment: 'Порвался ящик',
		});
	});

	it('в выбор партии не попадают просроченные, первой расходуется годная', () => {
		expect(usableBatches([expired, batch()], now).map(({ id }) => id)).toEqual([
			'b-111',
		]);
		expect(firstInLine([expired, batch()], now)).toBe('b-111');
	});
});

describe('исправление остатка', () => {
	it.each([
		['верное', { actual: 1400, reason: 'Пересчёт', comment: '' }, {}],
		[
			'без количества',
			{ actual: null, reason: 'Пересчёт', comment: '' },
			{ actual: 'Укажите, сколько партии лежит на самом деле' },
		],
		[
			'без причины',
			{ actual: 1400, reason: ' ', comment: '' },
			{ reason: 'Укажите причину' },
		],
	])('%s', (_, values, errors) => {
		expect(adjustProblems(values)).toEqual(errors);
	});

	it('изменение считается от остатка партии', () => {
		expect(
			toAdjustment(
				{ actual: 1400, reason: ' Пересчёт ', comment: ' нашли ' },
				batch()
			)
		).toEqual({ delta: -200, reason: 'Пересчёт', comment: 'нашли' });
		expect(
			toAdjustment({ actual: 1700, reason: 'Другое', comment: '' }, batch())
		).toEqual({ delta: 100, reason: 'Другое' });
	});
});
