import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { lastParams } from './test-api';
import {
	deliveriesApi,
	testDelivery,
	testDeliveryItem,
	testSummary,
} from './test-deliveries';
import { renderApp, testPoint, testProfile } from './test-router';

describe('список поставок', () => {
	it('параметры адреса уходят в запрос списка, неверный размер страницы заменяется на 10', async () => {
		const get = deliveriesApi({
			list: () => ({ items: [testDelivery()], total: 60 }),
		});

		await renderApp(
			'/points/lenina/deliveries?status="registered"&page=2&size=25'
		);
		await waitFor(() =>
			expect(lastParams(get, '/deliveries')).toMatchObject({
				status: 'registered',
				limit: 25,
				offset: 25,
			})
		);

		get.mockClear();
		await renderApp('/points/lenina/deliveries?size=500');
		await waitFor(() =>
			expect(lastParams(get, '/deliveries')).toMatchObject({
				limit: 10,
				offset: 0,
			})
		);
	});

	it('поиск отправляет начало номера и открывает первую страницу', async () => {
		const get = deliveriesApi();
		const { router } = await renderApp('/points/lenina/deliveries?page=3');

		await userEvent.type(
			await screen.findByRole('textbox', { name: 'Поиск по номеру поставки' }),
			'11'
		);

		await waitFor(() =>
			expect(lastParams(get, '/deliveries')).toMatchObject({
				number: '11',
				offset: 0,
			})
		);
		expect(router.state.location.search).toEqual({ number: '11' });
	});

	it('смена числа строк сохраняется в адресе и открывает первую страницу', async () => {
		deliveriesApi({
			list: () => ({ items: [testDelivery()], total: 118 }),
		});
		const { router } = await renderApp('/points/lenina/deliveries?page=3');

		await userEvent.click(
			await screen.findByRole('combobox', { name: /Строк на странице/ })
		);
		await userEvent.click(screen.getByRole('option', { name: '25' }));

		await waitFor(() =>
			expect(router.state.location.search).toEqual({ size: 25 })
		);
	});

	it('строки показывают просрочку ожидающей и «Генератор» у поставки генератора', async () => {
		deliveriesApi({
			list: [
				testDelivery({
					items: [
						testDeliveryItem({ expires_at: '2020-01-01T00:00:00Z' }),
						testDeliveryItem({ id: 'b' }),
					],
				}),
				testDelivery({
					id: 'delivery-119',
					number: '119',
					status: 'approved',
					approved_at: '2026-10-10T16:06:00Z',
					approved_by: { type: 'service', id: 'goden-generator' },
					items: [testDeliveryItem({ accepted_quantity: 4000 })],
				}),
			],
		});
		await renderApp('/points/lenina/deliveries');

		const table = await screen.findByRole('table', { name: 'Список поставок' });
		const pending = within(table).getByRole('row', { name: /№ 118/ });
		const generated = within(table).getByRole('row', { name: /№ 119/ });

		expect(within(pending).getByText('1 просрочена')).toBeInTheDocument();
		expect(within(generated).getByText('Генератор')).toBeInTheDocument();
	});

	it('ничего не найдено: «Сбросить поиск» возвращает весь список', async () => {
		deliveriesApi({
			list: (params) =>
				params.number
					? { items: [], total: 0 }
					: { items: [testDelivery()], total: 1 },
		});
		await renderApp('/points/lenina/deliveries?number="12А"');

		expect(
			await screen.findByText('Нет поставок с номером на «12А»')
		).toBeInTheDocument();

		await userEvent.click(
			screen.getByRole('button', { name: 'Сбросить поиск' })
		);

		expect(await screen.findByText('№ 118')).toBeInTheDocument();
		expect(
			screen.getByRole('textbox', { name: 'Поиск по номеру поставки' })
		).toHaveValue('');
	});

	it('ошибка загрузки показывает текст и «Повторить» перезапрашивает список', async () => {
		let fail = true;

		deliveriesApi({
			summary: testSummary(),
			list: () => {
				if (fail) {
					throw new Error(
						'Сервис временно недоступен. Повторите попытку позже.'
					);
				}

				return { items: [testDelivery()], total: 1 };
			},
		});
		await renderApp('/points/lenina/deliveries');

		expect(
			await screen.findByText(
				'Сервис временно недоступен. Повторите попытку позже.'
			)
		).toBeInTheDocument();

		fail = false;
		await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));

		expect(await screen.findByText('№ 118')).toBeInTheDocument();
	});

	it('смена точки на карточке поставки открывает список поставок новой точки', async () => {
		deliveriesApi();
		const { router } = await renderApp(
			'/points/lenina/deliveries/delivery-118',
			{
				profile: testProfile([
					testPoint(),
					testPoint({ id: 'mira', name: 'Точка на Мира' }),
				]),
			}
		);

		await userEvent.click(
			await screen.findByRole('button', { name: /Сменить точку/ })
		);
		await userEvent.click(
			screen.getByRole('menuitemradio', { name: /Точка на Мира/ })
		);

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/mira/deliveries')
		);
	});
});
