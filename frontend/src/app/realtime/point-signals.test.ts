import { QueryClient } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { deliveryKeys } from '@entities/delivery';
import { dishKeys, hotDishKeys } from '@entities/dish';
import { meQueryOptions } from '@entities/profile';
import { stockKeys } from '@entities/stock';
import { orderKeys } from '@pages/orders';
import {
	catalogKey,
	listRefreshDelayMs,
	pointDataFreshMs,
} from '@shared/lib/query';

import { testPoint, testProfile } from '../test-router';

import { refreshAfterSync, refreshCategories } from './point-signals';

const keys = {
	kitchen: dishKeys.menu('lenina'),
	stock: stockKeys.list('lenina', { sort: 'name', limit: 10, offset: 0 }),
	deliveries: deliveryKeys.detail('lenina', '118'),
	otherPoint: dishKeys.menu('mira'),
};

let queryClient: QueryClient;

const invalidated = () =>
	Object.entries(keys)
		.filter(
			([, queryKey]) => queryClient.getQueryState(queryKey)?.isInvalidated
		)
		.map(([name]) => name);

beforeEach(() => {
	queryClient = new QueryClient();
	for (const queryKey of Object.values(keys)) {
		queryClient.setQueryData(queryKey, {});
	}
});

describe('refreshCategories', () => {
	it('stop-list помечает кухню своей точки и не трогает поставки и другую точку', () => {
		refreshCategories(queryClient, 'lenina', ['stop-list']);

		expect(invalidated()).toEqual(['kitchen']);
	});

	it('stock и expiry в одном сигнале перечитывают склад один раз', () => {
		const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

		refreshCategories(queryClient, 'lenina', ['stock', 'expiry', 'unknown']);

		expect(invalidated()).toEqual(['stock']);
		expect(invalidate).toHaveBeenCalledOnce();
	});

	it('orders помечает историю заказов только после задержки и не трогает остальное', () => {
		vi.useFakeTimers();
		const history = orderKeys.list('lenina', { limit: 10, offset: 0 });

		queryClient.setQueryData(history, {});
		refreshCategories(queryClient, 'lenina', ['orders']);

		expect(queryClient.getQueryState(history)?.isInvalidated).toBe(false);

		vi.advanceTimersByTime(listRefreshDelayMs);

		expect(queryClient.getQueryState(history)?.isInvalidated).toBe(true);
		expect(invalidated()).toEqual([]);
	});

	it('catalog перечитывает каталог компании точки, её склад и меню, но не другую компанию', () => {
		const catalog = [...catalogKey('demo'), 'products'];
		const otherCompany = [...catalogKey('other'), 'products'];

		queryClient.setQueryData(
			meQueryOptions.queryKey,
			testProfile([testPoint()])
		);
		queryClient.setQueryData(catalog, {});
		queryClient.setQueryData(otherCompany, {});

		refreshCategories(queryClient, 'lenina', ['catalog']);

		expect(queryClient.getQueryState(catalog)?.isInvalidated).toBe(true);
		expect(queryClient.getQueryState(otherCompany)?.isInvalidated).toBe(false);
		expect(invalidated()).toEqual(['kitchen', 'stock']);
	});
});

describe('refreshCategories: горячие предложения', () => {
	it('recommendations перечитывает и ручные настройки предложений', () => {
		const settings = hotDishKeys.settings('lenina');

		queryClient.setQueryData(settings, []);
		refreshCategories(queryClient, 'lenina', ['recommendations']);

		expect(queryClient.getQueryState(settings)?.isInvalidated).toBe(true);
	});
});

describe('refreshAfterSync', () => {
	it('после первого подключения перечитывает только данные старше 15 секунд', () => {
		queryClient.setQueryData(
			keys.stock,
			{},
			{
				updatedAt: Date.now() - pointDataFreshMs - 1,
			}
		);

		refreshAfterSync(queryClient, 'lenina', { reconnect: false });

		expect(invalidated()).toEqual(['stock']);
	});

	it('после переподключения перечитывает все данные точки', () => {
		refreshAfterSync(queryClient, 'lenina', { reconnect: true });

		expect(invalidated()).toEqual(['kitchen', 'stock', 'deliveries']);
	});
});
