import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import type { HotDish } from '@entities/dish';
import { apiClient, ApiError } from '@shared/api';
import { withUnits } from '@shared/lib/test-utils';

import { idempotencyKeyOf, requestsTo } from './test-api';
import {
	bruschetta,
	carbonara,
	testDish,
	testIngredient,
} from './test-kitchen';
import {
	cream,
	hotCarbonara,
	pinnedHot,
	ordersApi,
	testOrder,
} from './test-orders';
import { manager, renderApp, testProfile, waiter } from './test-router';

const post = vi.spyOn(apiClient, 'post');

const caesar = testDish('Салат Цезарь с курицей', [
	testIngredient('Салат романо', 'g', 80, 100),
]);

/** Карбонара после чужих заказов: сливок осталось на одну порцию. */
const carbonaraOnePortion = testDish('Паста карбонара', [
	testIngredient('Спагетти', 'g', 120, 4800),
	testIngredient('Сливки 33%', 'ml', 80, 90),
]);

async function openForm(hot: HotDish[] = [hotCarbonara()]) {
	const state = { menu: [carbonara, bruschetta, caesar], hot };
	const get = ordersApi(state);

	await renderApp('/points/lenina/orders/new', {
		profile: testProfile([waiter]),
	});
	await screen.findByRole('table', { name: 'Меню для заказа' });
	return { get, state };
}

const more = (dish: string) =>
	userEvent.click(
		screen.getByRole('button', { name: `${dish}: порций: больше` })
	);

const portionsOf = (dish: string) =>
	screen.getByRole('textbox', { name: `${dish}: порций` });

const submit = () =>
	userEvent.click(screen.getByRole('button', { name: 'Оформить заказ' }));

describe('новый заказ по меню', () => {
	it('порций не больше, чем можно приготовить; блюдо стоп-листа не выбирается; горячее помечено', async () => {
		await openForm();

		expect(
			screen.getByRole('button', { name: 'Оформить заказ' })
		).toBeDisabled();

		await more('Салат Цезарь с курицей');

		expect(portionsOf('Салат Цезарь с курицей')).toHaveValue('1');
		expect(screen.getByText('больше не приготовить')).toBeVisible();
		expect(
			screen.getByRole('button', {
				name: 'Салат Цезарь с курицей: порций: больше',
			})
		).toBeDisabled();
		expect(portionsOf('Брускетта с томатами')).toBeDisabled();
		expect(
			screen.getByText(
				withUnits('Не хватает: Томаты · есть 60 г, на порцию 100 г')
			)
		).toBeVisible();

		const pasta = screen.getByRole('row', { name: /Паста карбонара/ });

		expect(within(pasta).getByText('предложить')).toBeVisible();
		expect(within(pasta).getByText('Сливки 33% · ещё 6 ч')).toBeVisible();
	});

	it('закреплённое блюдо отмечено «закреплено» с комментарием управляющего', async () => {
		await openForm([pinnedHot(hotCarbonara(), 'Новинка недели')]);

		const pasta = await screen.findByRole('row', { name: /Паста карбонара/ });

		expect(within(pasta).getByText('закреплено')).toBeVisible();
		expect(within(pasta).getByText('«Новинка недели»')).toBeVisible();
		expect(within(pasta).queryByText('предложить')).toBeNull();
	});

	it('принятый заказ: состав и ключ уходят на сервер, порции очищены, меню перечитано', async () => {
		post.mockResolvedValueOnce({
			data: testOrder({
				items: [
					{
						dish_id: carbonara.dish.id,
						dish_name: 'Паста карбонара',
						portions: 2,
					},
				],
			}),
		});
		const { get } = await openForm();
		const menuBefore = requestsTo(get, '/menu').length;

		await more('Паста карбонара');
		await more('Паста карбонара');
		await submit();

		await waitFor(() => expect(post).toHaveBeenCalledOnce());
		expect(post.mock.calls[0][0]).toBe('/points/lenina/orders');
		expect(post.mock.calls[0][1]).toEqual({
			items: [{ dish_id: carbonara.dish.id, portions: 2 }],
		});
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));
		expect(await screen.findByText(/^Заказ принят в/)).toBeVisible();
		expect(
			screen.getByText(/Паста карбонара × 2\. Продукты списаны/)
		).toBeVisible();
		expect(portionsOf('Паста карбонара')).toHaveValue('0');
		await waitFor(() =>
			expect(requestsTo(get, '/menu').length).toBeGreaterThan(menuBefore)
		);
	});

	it('отклонённый заказ: продукт с нужным и доступным количеством, порции на месте, превышение отмечено', async () => {
		const { state } = await openForm();

		post.mockImplementationOnce(async () => {
			state.menu = [carbonaraOnePortion, bruschetta, caesar];
			return {
				data: testOrder({
					status: 'rejected',
					items: [
						{
							dish_id: carbonara.dish.id,
							dish_name: 'Паста карбонара',
							portions: 2,
						},
					],
					shortages: [
						{
							product_id: cream.id,
							product: cream,
							required_quantity: 160,
							available_quantity: 90,
						},
					],
				}),
			};
		});

		await more('Паста карбонара');
		await more('Паста карбонара');
		await submit();

		expect(
			await screen.findByText('Заказ отклонён: не хватило продуктов')
		).toBeVisible();
		expect(
			screen.getByText(withUnits('— нужно 160 мл, было 90 мл'), {
				exact: false,
			})
		).toBeVisible();
		expect(portionsOf('Паста карбонара')).toHaveValue('2');
		expect(await screen.findByText('можно только 1')).toBeVisible();
		expect(
			screen.getByRole('button', { name: 'Оформить заказ' })
		).toBeDisabled();
	});

	it('после обрыва «Повторить» отправляет заказ с тем же ключом', async () => {
		post
			.mockRejectedValueOnce(
				new ApiError('Нет связи с сервером', { status: 0 })
			)
			.mockResolvedValueOnce({ data: testOrder() });
		await openForm();

		await more('Салат Цезарь с курицей');
		await submit();

		expect(await screen.findByText('Заказ не оформлен')).toBeVisible();

		await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));

		await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});

	it('блюдо выключили на точке: оно названо и убрано, остальные порции на месте', async () => {
		const { state } = await openForm();

		post.mockImplementationOnce(async () => {
			state.menu = [carbonara, bruschetta];
			throw new ApiError(`блюдо ${caesar.dish.id} не продаётся на этой точке`, {
				status: 422,
				type: 'unknown-reference',
			});
		});

		await more('Паста карбонара');
		await more('Салат Цезарь с курицей');
		await submit();

		expect(
			await screen.findByText(
				'Салат Цезарь с курицей больше не продаётся на точке'
			)
		).toBeVisible();
		expect(portionsOf('Паста карбонара')).toHaveValue('1');
		expect(screen.queryByRole('row', { name: /Салат Цезарь/ })).toBeNull();
		expect(screen.queryByText('Заказ не оформлен')).toBeNull();
	});

	it('пустое меню точки: «В меню точки пока нет блюд», как на кухне', async () => {
		ordersApi({ menu: [], hot: [] });
		await renderApp('/points/lenina/orders/new', {
			profile: testProfile([waiter]),
		});

		expect(
			await screen.findByRole('heading', { name: 'В меню точки пока нет блюд' })
		).toBeVisible();
		expect(
			screen.getByText(/добавляет владелец компании в каталоге/)
		).toBeVisible();
	});

	it('без права оформлять заказы адрес формы показывает «Нет доступа»', async () => {
		ordersApi();
		await renderApp('/points/lenina/orders/new', {
			profile: testProfile([manager]),
		});

		expect(
			await screen.findByRole('heading', { name: 'Нет доступа к разделу' })
		).toBeVisible();
	});
});
