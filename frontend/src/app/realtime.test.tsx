import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { FakeEventSource } from '@shared/lib/test-utils';

import { deliveriesApi } from './test-deliveries';
import { kitchenApi, testMenu } from './test-kitchen';
import { hotCarbonara, ordersApi } from './test-orders';
import { mockGet, requestsTo } from './test-api';
import { connect, signal } from './test-realtime';
import { renderApp, testPoint, testProfile } from './test-router';

function drop() {
	act(() => {
		FakeEventSource.latest.fail();
	});
}

const wait = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

async function openKitchen(
	path = '/points/lenina/kitchen',
	profile = testProfile()
) {
	const get = kitchenApi({ menu: testMenu });
	const view = await renderApp(path, { profile });

	await screen.findByText('5 блюд в меню, 3 в стоп-листе');
	return { get, ...view };
}

beforeEach(() => {
	vi.useFakeTimers({ shouldAdvanceTime: true });
	vi.setSystemTime(new Date(2026, 9, 14, 14, 39, 40));
});

describe('поток точки: перечитывание', () => {
	it('на «Кухне» сигнал stock и stop-list перечитывает меню', async () => {
		const { get } = await openKitchen();

		connect();
		get.mockClear();
		signal(['stock', 'stop-list']);

		await waitFor(() => expect(requestsTo(get, '/menu')).toHaveLength(1));
	});

	it('первый sync не загружает свежие данные повторно, sync после обрыва загружает', async () => {
		const { get } = await openKitchen();

		get.mockClear();
		connect();
		await wait(100);

		expect(requestsTo(get, '/menu')).toHaveLength(0);

		drop();
		connect();

		await waitFor(() => expect(requestsTo(get, '/menu')).toHaveLength(1));
	});

	it('на «Поставках» сигнал deliveries перечитывает список через секунду, серия даёт один запрос', async () => {
		const get = deliveriesApi();

		await renderApp('/points/lenina/deliveries');
		await waitFor(() => expect(requestsTo(get, '/deliveries')).toHaveLength(1));
		get.mockClear();
		connect();
		signal(['stock']);
		await wait(2000);

		expect(requestsTo(get, '/deliveries')).toHaveLength(0);

		signal(['deliveries']);
		await wait(200);
		signal(['deliveries']);
		await wait(200);
		signal(['deliveries']);
		await wait(900);

		expect(requestsTo(get, '/deliveries')).toHaveLength(0);

		await wait(200);

		await waitFor(() => expect(requestsTo(get, '/deliveries')).toHaveLength(1));
		expect(requestsTo(get, '/deliveries/summary')).toHaveLength(1);
	});

	it('в истории заказов сигнал orders перечитывает список через секунду, серия даёт один запрос', async () => {
		const get = ordersApi({ hot: [hotCarbonara()] });

		await renderApp('/points/lenina/orders');
		await waitFor(() => expect(requestsTo(get, '/orders')).toHaveLength(1));
		connect();
		await wait(100);
		get.mockClear();

		signal(['orders']);
		await wait(200);
		signal(['orders']);
		await wait(200);
		signal(['orders']);
		await wait(900);

		expect(requestsTo(get, '/orders')).toHaveLength(0);

		await wait(200);

		await waitFor(() => expect(requestsTo(get, '/orders')).toHaveLength(1));
		expect(requestsTo(get, '/recommendations/dishes')).toHaveLength(0);
	});

	it('открытая «Аналитика» перечитывается через секунду после stock и не перечитывается после deliveries', async () => {
		const get = mockGet(() => ({ items: [] }));

		await renderApp('/points/lenina/analytics');
		await waitFor(() =>
			expect(requestsTo(get, '/stop-list-forecast')).toHaveLength(1)
		);
		connect();
		await wait(100);
		get.mockClear();

		signal(['deliveries']);
		await wait(2000);

		expect(requestsTo(get, '/stop-list-forecast')).toHaveLength(0);

		signal(['stock']);
		await wait(900);

		expect(requestsTo(get, '/stop-list-forecast')).toHaveLength(0);

		await wait(200);

		await waitFor(() =>
			expect(requestsTo(get, '/stop-list-forecast')).toHaveLength(1)
		);
		expect(requestsTo(get, '/shortages')).toHaveLength(1);
	});
});

describe('поток точки: связь в оболочке', () => {
	it('индикатор показывает время сигнала, короткий обрыв проходит без плашки', async () => {
		await openKitchen();
		connect();

		expect(screen.getByText('в сети · обновлено 14:39')).toBeInTheDocument();

		await wait(20_000);
		signal(['recommendations']);

		expect(screen.getByText('в сети · обновлено 14:40')).toBeInTheDocument();

		drop();
		await wait(3000);
		connect();
		await wait(5000);

		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('без связи дольше 5 секунд показывает плашку, «Повторить» подключается заново', async () => {
		const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

		await openKitchen();
		connect();
		await wait(20_000);
		drop();
		await wait(4900);

		expect(screen.queryByRole('alert')).toBeNull();

		await wait(200);

		expect(screen.getByRole('alert')).toHaveTextContent(
			'Нет связи с сервером. Данные на 14:40 — переподключаемся…'
		);

		await user.click(screen.getByRole('button', { name: 'Повторить' }));

		expect(FakeEventSource.instances).toHaveLength(2);
		expect(FakeEventSource.active()).toEqual([FakeEventSource.latest]);

		connect();

		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('переход внутри точки не переподключает поток, смена точки переподключает', async () => {
		const { router } = await openKitchen(
			'/points/lenina/kitchen',
			testProfile([
				testPoint(),
				testPoint({ id: 'mira', name: 'Точка на Мира' }),
			])
		);
		const lenina = FakeEventSource.latest;

		await act(() =>
			router.navigate({
				to: '/points/$pointId/stop-list',
				params: { pointId: 'lenina' },
			})
		);

		expect(FakeEventSource.instances).toEqual([lenina]);

		await act(() =>
			router.navigate({
				to: '/points/$pointId/kitchen',
				params: { pointId: 'mira' },
			})
		);

		expect(lenina.readyState).toBe(FakeEventSource.CLOSED);
		expect(FakeEventSource.active()).toHaveLength(1);
		expect(FakeEventSource.latest.url).toBe('/api/v1/points/mira/events');
	});
});
