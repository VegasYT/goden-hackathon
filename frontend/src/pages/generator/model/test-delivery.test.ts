import { describe, expect, it } from 'vitest';

import type { Product } from '@entities/product';
import { testProduct } from '@entities/product/index.testing';
import { buildTestDelivery } from './test-delivery';

const now = new Date(2026, 9, 13, 14, 32, 5);
const hour = 3_600_000;
const day = 24 * hour;

const product = (id: string, overrides: Partial<Product> = {}) =>
	testProduct({ id, name: id, ...overrides });

const catalog = ['a', 'b', 'c', 'd', 'e', 'f'].map((id) => product(id));

/** Детерминированная последовательность от 0 до 1. */
function sequence(seed = 1) {
	let state = seed;

	return () => {
		state = (state * 9301 + 49297) % 233280;

		return state / 233280;
	};
}

const expiry = (iso: string) => new Date(iso).getTime() - now.getTime();

describe('buildTestDelivery', () => {
	it('берёт разные продукты и нумерует поставку «ТЕСТ-ММДД-ЧЧММСС»', () => {
		const delivery = buildTestDelivery(
			catalog,
			{ count: 5, expired: false, shortLife: false },
			now,
			sequence()
		);

		expect(delivery?.number).toBe('ТЕСТ-1013-143205');
		expect(delivery?.delivered_at).toBe(now.toISOString());
		expect(new Set(delivery?.items.map((item) => item.product_id)).size).toBe(
			5
		);
	});

	it('добавляет проблемные позиции к выбранным: просроченная сутки назад, короткая через 10 минут', () => {
		const delivery = buildTestDelivery(
			catalog,
			{ count: 3, expired: true, shortLife: true },
			now,
			sequence()
		);
		const expiries = delivery?.items.map((item) => expiry(item.expires_at));

		expect(delivery?.items).toHaveLength(5);
		expect(expiries).toContain(-day);
		expect(expiries).toContain(10 * 60_000);
	});

	it('при маленьком каталоге сокращает обычные позиции, а проблемную оставляет', () => {
		const delivery = buildTestDelivery(
			catalog.slice(0, 2),
			{ count: 3, expired: true, shortLife: false },
			now,
			sequence()
		);
		const expiries = delivery?.items.map((item) => expiry(item.expires_at));

		expect(expiries).toHaveLength(2);
		expect(expiries?.filter((value) => value === -day)).toHaveLength(1);
		expect(expiries?.filter((value) => value > 0)).toHaveLength(1);
	});

	it('срок годной позиции не выходит за срок хранения, без срока хранения — за 7 суток', () => {
		for (let seed = 1; seed < 50; seed++) {
			const delivery = buildTestDelivery(
				[
					product('short', { shelf_life_days: 3 }),
					product('none', { shelf_life_days: undefined }),
				],
				{ count: 2, expired: false, shortLife: false },
				now,
				sequence(seed)
			);

			for (const item of delivery?.items ?? []) {
				const limit = item.product_id === 'short' ? 3 * day : 7 * day;
				const left = expiry(item.expires_at);

				expect(left).toBeGreaterThan(0);
				expect(left).toBeLessThanOrEqual(limit);
			}
		}
	});

	it('количество целое: граммы от 1 до 5 кг с шагом 100 г, штуки от 10 до 50', () => {
		for (let seed = 1; seed < 50; seed++) {
			const delivery = buildTestDelivery(
				[product('grams'), product('pieces', { unit: 'pcs' })],
				{ count: 2, expired: false, shortLife: false },
				now,
				sequence(seed)
			);

			for (const item of delivery?.items ?? []) {
				const [min, max, step] =
					item.product_id === 'pieces' ? [10, 50, 1] : [1000, 5000, 100];

				expect(item.declared_quantity).toBeGreaterThanOrEqual(min);
				expect(item.declared_quantity).toBeLessThanOrEqual(max);
				expect(item.declared_quantity % step).toBe(0);
			}
		}
	});

	it('пустой каталог даёт null', () => {
		expect(
			buildTestDelivery([], { count: 3, expired: true, shortLife: false }, now)
		).toBeNull();
	});
});
