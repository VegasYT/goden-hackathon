import { QueryClient } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
	deliveryKeys,
	type Delivery,
	type DeliveryListParams,
	type DeliveryPage,
} from '../api/delivery-queries';
import { listRefreshDelayMs } from '@shared/lib/query';

import { testDelivery } from '../index.testing';
import { refreshDeliveries, storeDelivery } from './cache';

const params: DeliveryListParams = {
	sort: '-delivered_at',
	limit: 10,
	offset: 0,
};

function delivery(id: string, status: Delivery['status']): Delivery {
	return testDelivery({ id, number: id, status, items: [] });
}

function page(...items: Delivery[]): DeliveryPage {
	return { items, total: items.length };
}

describe('storeDelivery', () => {
	let queryClient: QueryClient;

	beforeEach(() => {
		vi.useFakeTimers();
		queryClient = new QueryClient();
	});

	it('заменяет строку в списках своей точки, не трогая другую точку', () => {
		const other = deliveryKeys.list('mira', params);

		queryClient.setQueryData(
			deliveryKeys.list('lenina', params),
			page(delivery('118', 'registered'), delivery('117', 'approved'))
		);
		queryClient.setQueryData(other, page(delivery('118', 'registered')));

		storeDelivery(queryClient, 'lenina', delivery('118', 'approved'));

		expect(
			queryClient
				.getQueryData<DeliveryPage>(deliveryKeys.list('lenina', params))
				?.items.map((item) => item.status)
		).toEqual(['approved', 'approved']);
		expect(queryClient.getQueryData<DeliveryPage>(other)?.items[0].status).toBe(
			'registered'
		);
		expect(
			queryClient.getQueryData(deliveryKeys.detail('lenina', '118'))
		).toMatchObject({ status: 'approved' });
	});

	it('помечает списки и сводку устаревшими только после задержки', () => {
		queryClient.setQueryData(deliveryKeys.list('lenina', params), page());
		queryClient.setQueryData(deliveryKeys.summary('lenina'), {});

		storeDelivery(queryClient, 'lenina', delivery('118', 'approved'));

		const stale = () =>
			[deliveryKeys.list('lenina', params), deliveryKeys.summary('lenina')].map(
				(queryKey) => queryClient.getQueryState(queryKey)?.isInvalidated
			);

		expect(stale()).toEqual([false, false]);

		vi.advanceTimersByTime(listRefreshDelayMs);

		expect(stale()).toEqual([true, true]);
	});

	it('сразу помечает устаревшей историю этой поставки, не трогая другие', () => {
		queryClient.setQueryData(deliveryKeys.history('lenina', '118'), {});
		queryClient.setQueryData(deliveryKeys.history('lenina', '117'), {});

		storeDelivery(queryClient, 'lenina', delivery('118', 'approved'));

		expect(
			queryClient.getQueryState(deliveryKeys.history('lenina', '118'))
				?.isInvalidated
		).toBe(true);
		expect(
			queryClient.getQueryState(deliveryKeys.history('lenina', '117'))
				?.isInvalidated
		).toBe(false);
	});
});

describe('refreshDeliveries', () => {
	let queryClient: QueryClient;

	beforeEach(() => {
		vi.useFakeTimers();
		queryClient = new QueryClient();
	});

	it('сразу помечает устаревшими карточки и историю, а списки и сводку через задержку', () => {
		const keys = [
			deliveryKeys.detail('lenina', '118'),
			deliveryKeys.history('lenina', '118'),
			deliveryKeys.list('lenina', params),
			deliveryKeys.summary('lenina'),
		];

		for (const queryKey of keys) {
			queryClient.setQueryData(queryKey, {});
		}

		const stale = () =>
			keys.map(
				(queryKey) => queryClient.getQueryState(queryKey)?.isInvalidated
			);

		refreshDeliveries(queryClient, 'lenina');

		expect(stale()).toEqual([true, true, false, false]);

		vi.advanceTimersByTime(listRefreshDelayMs);

		expect(stale()).toEqual([true, true, true, true]);
	});

	it('серия сигналов перечитывает списки один раз', () => {
		const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

		refreshDeliveries(queryClient, 'lenina');
		refreshDeliveries(queryClient, 'lenina');
		refreshDeliveries(queryClient, 'lenina');
		vi.advanceTimersByTime(listRefreshDelayMs);

		expect(
			invalidate.mock.calls.filter(
				([filters]) => filters?.queryKey?.at(-1) === 'list'
			)
		).toHaveLength(1);
	});
});
