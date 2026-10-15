import type { DeliveryListParams, NewDelivery } from '@entities/delivery';
import type { Product } from '@entities/product';
import { dayMs, minuteMs } from '@shared/lib/datetime';

/** Начало номера тестовых поставок: по нему их находит поиск списка. */
export const testNumberPrefix = 'ТЕСТ-';

/** Запрос числа тестовых поставок, ждущих приёмки. */
export const waitingTestDeliveries: DeliveryListParams = {
	status: 'registered',
	number: testNumberPrefix,
	sort: '-delivered_at',
	limit: 1,
	offset: 0,
};

export interface TestDeliveryOptions {
	/** Обычных позиций, от 1 до 5. */
	count: number;
	/** Добавить позицию, срок которой истёк сутки назад. */
	expired: boolean;
	/** Добавить позицию, срок которой истечёт через 10 минут. */
	shortLife: boolean;
}

const defaultShelfLifeDays = 7;
const minRemainingShare = 0.15;

const pad = (value: number) => String(value).padStart(2, '0');

function testNumber(now: Date): string {
	return `${testNumberPrefix}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
}

function shuffled<T>(items: T[], random: () => number): T[] {
	const result = [...items];

	for (let i = result.length - 1; i > 0; i--) {
		const j = Math.floor(random() * (i + 1));

		[result[i], result[j]] = [result[j], result[i]];
	}

	return result;
}

/** Целое число от `min` до `max` включительно. */
function between(min: number, max: number, random: () => number): number {
	return min + Math.floor(random() * (max - min + 1));
}

function quantity(product: Product, random: () => number): number {
	return between(10, 50, random) * (product.unit === 'pcs' ? 1 : 100);
}

/** Срок годной позиции: случайная доля срока хранения, как у генератора событий. */
function freshExpiry(product: Product, now: Date, random: () => number): Date {
	const shelfLife = (product.shelf_life_days ?? defaultShelfLifeDays) * dayMs;
	const remaining =
		shelfLife * (minRemainingShare + random() * (1 - minRemainingShare));

	return new Date(
		Math.floor((now.getTime() + remaining) / minuteMs) * minuteMs
	);
}

/** Тестовая поставка из разных случайных продуктов каталога или `null` при пустом каталоге; при нехватке продуктов сокращаются обычные позиции. */
export function buildTestDelivery(
	products: Product[],
	options: TestDeliveryOptions,
	now: Date,
	random: () => number = Math.random
): NewDelivery | null {
	if (products.length === 0) {
		return null;
	}

	const problemExpiries = [
		...(options.expired ? [new Date(now.getTime() - dayMs)] : []),
		...(options.shortLife ? [new Date(now.getTime() + 10 * minuteMs)] : []),
	].slice(0, products.length);
	const picked = shuffled(products, random);
	const problems = picked.slice(0, problemExpiries.length);
	const regular = picked.slice(
		problems.length,
		problems.length + options.count
	);

	return {
		number: testNumber(now),
		delivered_at: now.toISOString(),
		items: [
			...regular.map((product) => ({
				product_id: product.id,
				declared_quantity: quantity(product, random),
				expires_at: freshExpiry(product, now, random).toISOString(),
			})),
			...problems.map((product, index) => ({
				product_id: product.id,
				declared_quantity: quantity(product, random),
				expires_at: problemExpiries[index].toISOString(),
			})),
		],
	};
}
