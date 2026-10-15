import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import type { Product } from '@entities/product';
import { testProduct } from '@entities/product/index.testing';
import type { ExpiringBatch } from '@entities/stock';
import type { Batch, Movement, StockLevel, StockSummary } from '@pages/stock';
import { ApiError } from '@shared/api';

import { mockGet } from './test-api';
import { renderApp, testProfile } from './test-router';

export { testProduct } from '@entities/product/index.testing';

export const tomato = testProduct({
	id: 'tomato',
	name: 'Томаты черри',
	category: 'Овощи и зелень',
	shelf_life_days: 6,
});

export const mozzarella = testProduct({
	id: 'mozzarella',
	name: 'Моцарелла',
	category: 'Молочные продукты',
	shelf_life_days: 7,
});

/** Томаты: три партии, 3,4 кг доступно. */
export const tomatoLevel: StockLevel = {
	product: tomato,
	available_quantity: 3400,
	expired_quantity: 0,
	batches: 2,
	nearest_expires_at: '2099-10-13T08:00:00Z',
};

/** Моцарелла без партий. */
export const mozzarellaLevel: StockLevel = {
	product: mozzarella,
	available_quantity: 0,
	expired_quantity: 0,
	batches: 0,
};

export function testBatch(overrides: Partial<Batch> = {}): Batch {
	return {
		id: 'b-111',
		point_id: 'lenina',
		product_id: 'tomato',
		delivery_id: 'd-111',
		delivery_number: '111',
		item_id: 'i-111',
		received_quantity: 2000,
		quantity: 1600,
		expires_at: '2099-10-13T08:00:00Z',
		received_at: '2026-10-08T15:51:00Z',
		...overrides,
	};
}

export const tomatoBatches = [
	testBatch(),
	testBatch({
		id: 'b-113',
		delivery_id: 'd-113',
		delivery_number: '113',
		item_id: 'i-113',
		received_quantity: 1800,
		quantity: 1800,
		expires_at: '2099-10-15T23:59:00Z',
	}),
];

export function testStockSummary(
	overrides: Partial<StockSummary> = {}
): StockSummary {
	return {
		total: 2,
		in_stock: 1,
		expiring: 0,
		expired: 0,
		out_of_stock: 1,
		...overrides,
	};
}

export function testMovement(overrides: Partial<Movement> = {}): Movement {
	return {
		id: 'm-1',
		type: 'consumed',
		occurred_at: '2026-10-12T14:20:00Z',
		actor: { type: 'service', id: 'goden-generator' },
		changes: [{ batch_id: 'b-111', delivery_number: '111', delta: -120 }],
		...overrides,
	};
}

interface StockApi {
	levels?: StockLevel[];
	summary?: StockSummary;
	batches?: () => Batch[];
	movements?: (offset: number) => { items: Movement[]; total: number };
	/** Ответ остатков; по умолчанию — `levels` целиком. */
	stock?: (params: Record<string, unknown>) => unknown;
	/** Истекающие партии; по умолчанию их нет. */
	expiring?: (params: Record<string, unknown>) => unknown;
	/** Рекомендации поставок; по умолчанию их нет. */
	recommendations?: (params: Record<string, unknown>) => unknown;
}

const emptyPage = () => ({ items: [], total: 0 });

/** Подменяет чтения склада точки `lenina` и каталога компании `demo`. */
export function stockApi({
	levels = [tomatoLevel, mozzarellaLevel],
	summary = testStockSummary(),
	batches = () => tomatoBatches,
	movements = () => ({ items: [testMovement()], total: 1 }),
	stock,
	expiring = emptyPage,
	recommendations = emptyPage,
}: StockApi = {}) {
	return mockGet((url, params) => {
		if (url.endsWith('/product-categories')) {
			return {
				items: ['Молочные продукты', 'Овощи и зелень', 'Рыба'].map((name) => ({
					name,
					products: 1,
				})),
			};
		}

		if (url.startsWith('/companies/')) {
			const product = [tomato, mozzarella].find(({ id }) =>
				url.endsWith(`/products/${id}`)
			);

			return product ?? { items: [tomato, mozzarella], total: 2 };
		}

		if (url.endsWith('/stock/summary')) {
			return summary;
		}

		if (url.endsWith('/stock')) {
			return stock ? stock(params) : { items: levels, total: levels.length };
		}

		if (url.endsWith('/batches/expiring')) {
			return expiring(params);
		}

		if (url.endsWith('/recommendations/deliveries')) {
			return recommendations(params);
		}

		if (url.endsWith('/batches')) {
			return { items: batches(), total: batches().length };
		}

		if (url.endsWith('/movements')) {
			return movements(Number(params.offset ?? 0));
		}

		throw new ApiError('Не найдено', { status: 404 });
	});
}

const hour = 3_600_000;

/** Момент через `hours` часов от текущего времени, отрицательное — в прошлом. */
export const inHours = (hours: number) =>
	new Date(Date.now() + hours * hour).toISOString();

/** Истекающая партия продукта; просрочена ли она, считается по её сроку. */
export function testExpiring(
	product: Product,
	batch: Partial<Batch>
): ExpiringBatch {
	return {
		product,
		batch: testBatch({ product_id: product.id, ...batch }),
		expired: new Date(batch.expires_at ?? 0).getTime() <= Date.now(),
	};
}

/** Раскрывает партии томатов в списке склада и ждёт их таблицу. */
export async function expandTomato() {
	await userEvent.click(
		await screen.findByRole('button', { name: 'Партии продукта Томаты черри' })
	);

	return screen.findByRole('table', {
		name: 'Партии продукта Томаты черри, в порядке расхода',
	});
}

/** Открывает склад точки `lenina` с раскрытыми партиями томатов. */
export async function openTomato(profile = testProfile()) {
	const view = await renderApp('/points/lenina/stock', { profile });

	await expandTomato();

	return view;
}

/** Заполняет диалог «Списать порчу»: 0,5 кг, повреждена упаковка; пустой `comment` оставляет комментарий пустым. */
export async function fillWriteOff(
	dialog: HTMLElement,
	comment = 'Порвался ящик'
) {
	await userEvent.type(
		within(dialog).getByRole('textbox', { name: /^Количество, кг/ }),
		'0,5'
	);
	await userEvent.tab();
	await userEvent.click(
		within(dialog).getByRole('radio', { name: 'Повреждена упаковка' })
	);

	if (comment) {
		await userEvent.click(
			within(dialog).getByRole('textbox', { name: /^Комментарий/ })
		);
		await userEvent.paste(comment);
	}
}
