import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { meQueryOptions } from '@entities/profile';
import { apiClient, ApiError } from '@shared/api';
import { permissions } from '@shared/auth';
import { withUnits } from '@shared/lib/test-utils';

import { idempotencyKeyOf, lastParams, requestsTo } from './test-api';
import {
	ilyaActor,
	migomActor,
	ordersApi,
	rejectedOrder,
	testOrder,
} from './test-orders';
import { testProduct } from './test-stock';
import {
	manager,
	renderApp,
	testPoint,
	testProfile,
	waiter,
} from './test-router';

describe('раздел заказов', () => {
	it('официант видит историю, горячие предложения и «Оформить вручную»', async () => {
		ordersApi();
		await renderApp('/points/lenina/orders', {
			profile: testProfile([waiter]),
		});

		expect(await screen.findByRole('tab', { name: 'История' })).toBeVisible();
		expect(
			screen.getByRole('tab', { name: /^Горячие предложения/ })
		).toBeVisible();
		expect(
			screen.getByRole('link', { name: 'Оформить вручную' })
		).toHaveAttribute('href', '/points/lenina/orders/new');
	});

	it('у управляющего без права оформлять заказы действия нет', async () => {
		ordersApi();
		await renderApp('/points/lenina/orders', {
			profile: testProfile([manager]),
		});

		await screen.findByRole('tab', { name: 'История' });
		expect(screen.queryByText('Оформить вручную')).toBeNull();
	});

	it('на отключённой точке оформить нельзя, история видна', async () => {
		ordersApi();
		await renderApp('/points/lenina/orders', {
			profile: testProfile([testPoint({ active: false })]),
		});

		expect(
			await screen.findByRole('button', { name: 'Оформить вручную' })
		).toBeDisabled();
		expect(
			screen.getByText(/Точка отключена — заказы на ней не оформляются/)
		).toBeVisible();
		expect(
			await screen.findByRole('table', { name: 'История заказов' })
		).toBeVisible();
	});

	it('на отключённой точке пояснение видит и сотрудник без права оформлять заказы', async () => {
		ordersApi();
		await renderApp('/points/lenina/orders', {
			profile: testProfile([{ ...manager, active: false }]),
		});

		expect(
			await screen.findByText(/Точка отключена — заказы на ней не оформляются/)
		).toBeVisible();
	});

	it('точку отключили, пока раздел открыт: после перечитанного профиля оформить нельзя', async () => {
		ordersApi();
		const { router, queryClient } = await renderApp('/points/lenina/orders', {
			profile: testProfile([waiter]),
		});

		await screen.findByRole('link', { name: 'Оформить вручную' });
		act(() =>
			queryClient.setQueryData(
				meQueryOptions.queryKey,
				testProfile([{ ...waiter, active: false }])
			)
		);

		expect(
			await screen.findByRole('button', { name: 'Оформить вручную' })
		).toBeDisabled();
		expect(
			screen.getByText(/Точка отключена — заказы на ней не оформляются/)
		).toBeVisible();
		expect(router.state.location.pathname).toBe('/points/lenina/orders');
	});
});

describe('история заказов', () => {
	it('фильтр «Отклонённые» уходит в запрос и хранится в адресе', async () => {
		const get = ordersApi();
		const { router } = await renderApp('/points/lenina/orders');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Отклонённые' })
		);

		await waitFor(() =>
			expect(lastParams(get, '/orders')).toMatchObject({ status: 'rejected' })
		);
		expect(router.state.location.search).toMatchObject({ status: 'rejected' });
	});

	it('раскрытый отказ показывает, сколько нужно, сколько было и сколько не хватило', async () => {
		const twoMissing = testOrder({
			id: 'order-two',
			status: 'rejected',
			shortages: [
				{
					product_id: 'chicken',
					product: testProduct({ id: 'chicken', name: 'Куриное филе' }),
					required_quantity: 180,
					available_quantity: 150,
				},
				{
					product_id: 'romaine',
					product: testProduct({ id: 'romaine', name: 'Салат романо' }),
					required_quantity: 160,
					available_quantity: 40,
				},
				{ product_id: 'gone', required_quantity: 2, available_quantity: 0 },
			],
		});

		ordersApi({ orders: [rejectedOrder, twoMissing] });
		await renderApp('/points/lenina/orders');

		expect(
			await screen.findByText(
				'Куриное филе, Салат романо, Продукт удалён из каталога'
			)
		).toBeVisible();

		await userEvent.click(
			screen.getByRole('button', { name: /^Заказ 14\.10, \d\d:31$/ })
		);

		const shortages = screen.getByRole('table', {
			name: 'Недостающие продукты',
		});
		const cream = within(shortages).getByRole('row', { name: /Сливки 33%/ });

		expect(within(cream).getByText(withUnits('160 мл'))).toBeVisible();
		expect(within(cream).getByText(withUnits('90 мл'))).toBeVisible();
		expect(within(cream).getByText(withUnits('70 мл'))).toBeVisible();
		expect(screen.getByText('2 порции')).toBeVisible();
	});

	it('заказ Мигома отмечен «Мигом», отменённый показывает, кто отменил и с каким комментарием', async () => {
		ordersApi({
			orders: [
				testOrder({
					id: 'order-cancelled',
					status: 'cancelled',
					placed_by: migomActor,
					cancelled_by: migomActor,
					cancelled_at: '2026-10-14T11:36:00Z',
					cancel_comment: 'Покупатель отменил доставку',
				}),
				testOrder({ id: 'order-waiter' }),
			],
		});
		await renderApp('/points/lenina/orders');

		const cancelled = await screen.findByRole('row', { name: /отменён/ });

		expect(
			within(cancelled)
				.getAllByText('Мигом')
				.map((node) => node.textContent)
		).toEqual(['Мигом', 'Мигом']);
		expect(cancelled).toHaveTextContent(
			/Отменил Мигом, 14\.10, \d\d:36 · «Покупатель отменил доставку»/
		);
		expect(within(cancelled).queryByText(/Не хватило/)).toBeNull();
		expect(screen.getByText('Ольга Смирнова')).toBeVisible();
	});

	it('пустой результат фильтра предлагает показать все заказы', async () => {
		const get = ordersApi({
			orders: (params) => (params.status === 'rejected' ? [] : [testOrder()]),
		});

		await renderApp('/points/lenina/orders?status=%22rejected%22');

		expect(await screen.findByText('Отклонённых заказов нет')).toBeVisible();

		await userEvent.click(
			screen.getByRole('button', { name: 'Показать все заказы' })
		);

		await waitFor(() =>
			expect(lastParams(get, '/orders')).not.toHaveProperty(
				'status',
				'rejected'
			)
		);
	});

	it('следующая страница запрашивает у сервера следующие 10 заказов', async () => {
		const get = ordersApi({ orders: [testOrder()], total: 248 });

		await renderApp('/points/lenina/orders');
		await userEvent.click(
			await screen.findByRole('button', { name: /следующую страницу/i })
		);

		await waitFor(() =>
			expect(lastParams(get, '/orders')).toMatchObject({
				limit: 10,
				offset: 10,
			})
		);
	});
});

describe('отмена заказа', () => {
	const post = vi.spyOn(apiClient, 'post');

	const openCancel = async () => {
		await userEvent.click(
			await screen.findByRole('button', { name: /^Действия: заказ/ })
		);
		await userEvent.click(
			screen.getByRole('menuitem', { name: /Отменить заказ/ })
		);

		return screen.getByRole('alertdialog');
	};

	it('отмена с комментарием уходит на сервер, строка сразу «отменён»', async () => {
		const order = testOrder();

		ordersApi({ orders: [order] });
		post.mockResolvedValueOnce({
			data: {
				...order,
				status: 'cancelled',
				cancelled_by: ilyaActor,
				cancelled_at: '2026-10-14T11:40:00Z',
				cancel_comment: 'Гость ушёл',
			},
		});
		await renderApp('/points/lenina/orders');

		const dialog = await openCancel();

		expect(within(dialog).getByText('Оформил Ольга Смирнова')).toBeVisible();
		expect(within(dialog).queryByText(/Заказ из Мигома/)).toBeNull();

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Комментарий' }),
			'Гость ушёл'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Отменить заказ' })
		);

		expect(
			await screen.findByText(
				/^Заказ от .* отменён, продукты вернулись на склад$/
			)
		).toBeVisible();
		expect(post).toHaveBeenCalledWith(
			'/points/lenina/orders/order-1/cancel',
			{ comment: 'Гость ушёл' },
			expect.anything()
		);
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));
		expect(screen.getByRole('row', { name: /отменён/ })).toHaveTextContent(
			/Отменил Илья Смирнов, 14\.10, \d\d:40 · «Гость ушёл»/
		);
	});

	it('заказ Мигома: предупреждение о покупателе, отказ сервера показан и история перечитана', async () => {
		const get = ordersApi({
			orders: [testOrder({ placed_by: migomActor })],
		});

		post.mockRejectedValueOnce(
			new ApiError('Заказ уже отменён', { status: 409 })
		);
		await renderApp('/points/lenina/orders');

		const dialog = await openCancel();

		expect(
			within(dialog).getByText(
				'Заказ из Мигома: покупатель получит отмену с этим комментарием.'
			)
		).toBeVisible();

		const before = requestsTo(get, '/orders').length;

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Отменить заказ' })
		);

		expect(await within(dialog).findByText('Заказ уже отменён')).toBeVisible();
		expect(post.mock.calls[0][1]).toBeUndefined();
		await waitFor(() =>
			expect(requestsTo(get, '/orders').length).toBeGreaterThan(before)
		);
	});

	it.each([
		[
			'без права отменять',
			testPoint({
				permissions: permissions.filter((p) => p !== 'orders.cancel'),
			}),
		],
		['на отключённой точке', testPoint({ active: false })],
	])('%s у заказов нет меню действий', async (_, point) => {
		ordersApi({ orders: [testOrder()] });
		await renderApp('/points/lenina/orders', { profile: testProfile([point]) });

		await screen.findByText('принят');
		expect(
			screen.queryByRole('button', { name: /^Действия: заказ/ })
		).toBeNull();
	});
});
