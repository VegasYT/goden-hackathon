import { describe, expect, it } from 'vitest';

import type { Product } from '@entities/product';
import { testProduct } from '@entities/product/index.testing';

import {
	emptyItem,
	formProblems,
	isItemExpired,
	itemProblems,
	serverProblems,
	toNewDelivery,
	type ItemValues,
} from './registration';

const product = (id: string, overrides: Partial<Product> = {}) =>
	testProduct({ id, name: id, ...overrides });

const salmon = product('salmon');
const eggs = product('eggs', { unit: 'pcs', shelf_life_days: 25 });
const products = new Map([salmon, eggs].map((p) => [p.id, p]));

function item(overrides: Partial<ItemValues> = {}): ItemValues {
	return {
		...emptyItem(),
		productId: 'salmon',
		quantity: 12,
		expiresAt: new Date(2026, 9, 17, 23, 59),
		...overrides,
	};
}

describe('itemProblems', () => {
	it('пустая позиция требует продукт и срок годности', () => {
		expect(itemProblems(emptyItem())).toEqual({
			product: 'Выберите продукт',
			expiresAt: 'Укажите срок годности',
		});
	});

	it.each([
		[eggs, 12.5, 'Только целые штуки'],
		[salmon, 0, 'Больше нуля'],
		[salmon, null, 'Укажите количество'],
		[salmon, 2e9, 'Слишком большое количество'],
	])('%s в количестве %s — «%s»', (p, quantity, error) => {
		expect(itemProblems(item({ productId: p.id, quantity }), p).quantity).toBe(
			error
		);
	});

	it('изготовление позже срока годности', () => {
		expect(
			itemProblems(
				item({
					productId: 'eggs',
					quantity: 360,
					expiresAt: new Date(2026, 10, 3, 23, 59),
					manufacturedAt: new Date(2026, 10, 5),
				}),
				eggs
			)
		).toEqual({ manufacturedAt: 'Позже срока годности' });
	});

	it('срок годности дольше срока хранения продукта', () => {
		expect(
			itemProblems(
				item({
					manufacturedAt: new Date(2026, 9, 11),
					expiresAt: new Date(2026, 9, 20, 23, 59),
				}),
				salmon
			)
		).toEqual({ expiresAt: 'Дольше срока хранения продукта: до 5 суток' });
	});

	it('неверная дата срока', () => {
		expect(itemProblems(item({ expiresAt: new Date(NaN) }), salmon)).toEqual({
			expiresAt: 'Неверная дата',
		});
	});
});

describe('formProblems', () => {
	it('пустой номер и нет времени поступления', () => {
		const errors = formProblems(
			{ number: '  ', deliveredAt: null, items: [item()] },
			products
		);

		expect(errors.number).toBe('Укажите номер поставки');
		expect(errors.deliveredAt).toBe('Укажите время поступления');
		expect(errors.items).toEqual([{}]);
	});
});

describe('isItemExpired', () => {
	it('срок раньше «сейчас» истёк', () => {
		const now = new Date(2026, 9, 13, 14, 20);

		expect(
			isItemExpired(item({ expiresAt: new Date(2026, 9, 12, 22) }), now)
		).toBe(true);
		expect(isItemExpired(item(), now)).toBe(false);
	});
});

describe('serverProblems', () => {
	it('раскладывает ошибки сервера по полям формы', () => {
		expect(
			serverProblems(
				{
					'items[1].expires_at': { message: 'Срок не задан' },
					number: { message: 'Пустой номер' },
				},
				2
			)
		).toEqual({
			number: 'Пустой номер',
			items: [{}, { expiresAt: 'Срок не задан' }],
		});
	});

	it('ошибка не по полю формы — null', () => {
		expect(serverProblems({ items: { message: 'Пусто' } }, 1)).toBeNull();
	});
});

describe('toNewDelivery', () => {
	it('переводит количество в целые единицы и даты в ISO', () => {
		const deliveredAt = new Date(2026, 9, 13, 14, 20);
		const madeAt = new Date(2026, 9, 12);

		expect(
			toNewDelivery(
				{
					number: ' Н-4512 ',
					deliveredAt,
					items: [
						item({ manufacturedAt: madeAt }),
						item({ productId: 'eggs', quantity: 360 }),
					],
				},
				products
			)
		).toEqual({
			number: 'Н-4512',
			delivered_at: deliveredAt.toISOString(),
			items: [
				{
					product_id: 'salmon',
					declared_quantity: 12000,
					expires_at: new Date(2026, 9, 17, 23, 59).toISOString(),
					manufactured_at: madeAt.toISOString(),
				},
				{
					product_id: 'eggs',
					declared_quantity: 360,
					expires_at: new Date(2026, 9, 17, 23, 59).toISOString(),
				},
			],
		});
	});

	it('без продукта, целого количества, срока или времени поступления — null', () => {
		const deliveredAt = new Date(2026, 9, 13, 14, 20);
		const body = (items: ItemValues[], at: Date | null = deliveredAt) =>
			toNewDelivery({ number: 'Н-1', deliveredAt: at, items }, products);

		expect(body([item({ productId: null })])).toBeNull();
		expect(body([item({ productId: 'eggs', quantity: 1.5 })])).toBeNull();
		expect(body([item({ expiresAt: null })])).toBeNull();
		expect(body([item()], null)).toBeNull();
	});
});
