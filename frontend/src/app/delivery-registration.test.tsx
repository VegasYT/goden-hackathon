import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import type { Product } from '@entities/product';
import { apiClient, ApiError } from '@shared/api';
import { retype, typeDate } from '@shared/lib/test-utils';

import { idempotencyKeyOf, lastParams, mockGet } from './test-api';
import { testDelivery, testDeliveryItem } from './test-deliveries';
import { testProduct } from './test-stock';
import { approver, manager, renderApp, testProfile } from './test-router';

const salmon = testProduct({
	id: 'salmon',
	name: 'Лосось охлаждённый',
	category: 'Рыба',
});
const eggs = testProduct({
	id: 'eggs',
	name: 'Яйцо куриное С0',
	unit: 'pcs',
	category: 'Яйца',
	shelf_life_days: 25,
});

const registered = testDelivery({
	id: 'delivery-4512',
	number: 'Н-4512',
	items: [testDeliveryItem({ product_name: 'Лосось охлаждённый' })],
});

const managerProfile = testProfile([manager]);
const approverProfile = testProfile([approver]);

const post = vi.spyOn(apiClient, 'post');

function api(products: Product[] = [salmon, eggs]) {
	return mockGet((url) => {
		if (url.endsWith('/products')) {
			return { items: products, total: products.length };
		}

		if (url.endsWith('/deliveries/summary')) {
			return { total: 0, registered: 0, approved: 0, open_quarantine_items: 0 };
		}

		if (url.endsWith('/deliveries')) {
			return { items: [], total: 0 };
		}

		if (url.endsWith(`/deliveries/${registered.id}`)) {
			return registered;
		}

		throw new ApiError('Не найдено', { status: 404 });
	});
}

async function openForm(products?: Product[]) {
	api(products);
	const view = await renderApp('/points/lenina/deliveries/new', {
		profile: managerProfile,
	});

	await screen.findByRole('heading', { level: 1, name: 'Новая поставка' });

	return view;
}

async function chooseProduct(position: number, name: string) {
	await userEvent.click(
		screen.getByRole('combobox', { name: `Позиция ${position}: продукт` })
	);
	await userEvent.click(
		await screen.findByRole('option', { name: new RegExp(name) })
	);
}

const setQuantity = (position: number, value: string) =>
	retype(
		screen.getByRole('textbox', {
			name: new RegExp(`^Позиция ${position}: количество`),
		}),
		value
	);

async function fillValid() {
	await userEvent.click(screen.getByLabelText('Номер поставки'));
	await userEvent.paste('Н-4512');
	await chooseProduct(1, 'Лосось охлаждённый');
	await setQuantity(1, '12');
	await typeDate('Позиция 1: годен до', '17.10.2099 23:59');
}

const submit = () =>
	userEvent.click(
		screen.getByRole('button', { name: 'Зарегистрировать поставку' })
	);

describe('вход в регистрацию поставки', () => {
	it('управляющий видит «Новая поставка» в списке и открывает форму', async () => {
		api();
		await renderApp('/points/lenina/deliveries', { profile: managerProfile });

		await userEvent.click(
			await screen.findByRole('link', { name: 'Новая поставка' })
		);

		expect(
			await screen.findByRole('heading', { level: 1, name: 'Новая поставка' })
		).toBeInTheDocument();
	});

	it('у приёмщика кнопки нет, а адрес формы показывает «Нет доступа»', async () => {
		api();
		const { router } = await renderApp('/points/lenina/deliveries', {
			profile: approverProfile,
		});

		await screen.findByRole('heading', { level: 1, name: 'Поставки' });
		expect(screen.queryByRole('link', { name: 'Новая поставка' })).toBeNull();

		await act(() =>
			router.navigate({
				to: '/points/$pointId/deliveries/new',
				params: { pointId: 'lenina' },
			})
		);

		expect(
			await screen.findByRole('heading', { name: 'Нет доступа к разделу' })
		).toBeInTheDocument();
	});
});

describe('форма новой поставки', () => {
	it('продукт в штуках меняет единицу количества', async () => {
		await openForm();

		await chooseProduct(1, 'Яйцо куриное С0');

		expect(
			screen.getByRole('textbox', { name: /^Позиция 1: количество, шт/ })
		).toBeInTheDocument();
	});

	it('продукт ищется на сервере по введённому тексту, выбранный показывает категорию и срок', async () => {
		const get = api();

		await openForm();

		await userEvent.type(
			screen.getByRole('combobox', { name: 'Позиция 1: продукт' }),
			'лос'
		);

		await waitFor(() =>
			expect(lastParams(get, '/products')).toEqual({
				q: 'лос',
				limit: 20,
				offset: 0,
			})
		);
		await userEvent.click(
			await screen.findByRole('option', { name: /Лосось охлаждённый/ })
		);

		expect(screen.getByText('Рыба · хранится до 5 суток')).toBeVisible();
	});

	it('пустая позиция показывает ошибки у полей и не отправляется', async () => {
		await openForm();

		await submit();

		expect(screen.getByText('Укажите номер поставки')).toBeVisible();
		expect(screen.getByText('Выберите продукт')).toBeVisible();
		expect(screen.getByText('Укажите срок годности')).toBeVisible();
		expect(
			screen.getByText('Исправьте 1 позицию, чтобы зарегистрировать поставку')
		).toBeVisible();
		expect(post).not.toHaveBeenCalled();
	});

	it('прошедший срок предупреждает, но не мешает отправке', async () => {
		post.mockResolvedValueOnce({ data: registered });
		await openForm();

		await userEvent.click(screen.getByLabelText('Номер поставки'));
		await userEvent.paste('Н-4512');
		await chooseProduct(1, 'Лосось охлаждённый');
		await setQuantity(1, '3');
		await typeDate('Позиция 1: годен до', '12.10.2020 22:00');

		expect(
			screen.getByText('срок уже истёк: при приёмке позицию отклонят')
		).toBeVisible();

		await submit();
		await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
	});

	it('при пустом каталоге показывает сообщение без полей позиций', async () => {
		await openForm([]);

		expect(
			screen.getByText('Зарегистрировать поставку пока нельзя')
		).toBeVisible();
		expect(
			screen.queryByRole('table', { name: 'Позиции поставки' })
		).toBeNull();
	});
});

describe('отправка новой поставки', () => {
	it('отправляет поставку с ключом и открывает её карточку с сообщением', async () => {
		post.mockResolvedValueOnce({ data: registered });
		const { router } = await openForm();

		await fillValid();
		await submit();

		await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
		expect(post.mock.calls[0][0]).toBe('/points/lenina/deliveries');
		expect(post.mock.calls[0][1]).toEqual({
			number: 'Н-4512',
			delivered_at: expect.any(String),
			items: [
				{
					product_id: 'salmon',
					declared_quantity: 12000,
					expires_at: new Date(2099, 9, 17, 23, 59).toISOString(),
				},
			],
		});
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe(
				'/points/lenina/deliveries/delivery-4512'
			)
		);
		expect(
			await screen.findByText('Поставка № Н-4512 зарегистрирована · 1 позиция')
		).toBeVisible();
	});

	it('ошибку сервера у второй позиции показывает у её срока', async () => {
		post.mockRejectedValueOnce(
			new ApiError('Ошибка проверки', {
				status: 422,
				fieldErrors: {
					'items[1].expires_at': { message: 'Срок годности обязателен' },
				},
			})
		);
		await openForm();

		await fillValid();
		await userEvent.click(
			screen.getByRole('button', { name: 'Добавить позицию' })
		);
		await chooseProduct(2, 'Яйцо куриное С0');
		await setQuantity(2, '360');
		await typeDate('Позиция 2: годен до', '03.11.2099 23:59');
		await submit();

		const row = await screen.findByRole('row', {
			name: /Срок годности обязателен/,
		});

		expect(within(row).getByText('2')).toBeInTheDocument();
	});

	it('после сбоя «Повторить» отправляет ту же поставку с тем же ключом', async () => {
		post
			.mockRejectedValueOnce(new ApiError('Нет связи с сервером'))
			.mockResolvedValueOnce({ data: registered });
		await openForm();

		await fillValid();
		await submit();
		await userEvent.click(
			await screen.findByRole('button', { name: 'Повторить' })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
		expect(post.mock.calls[1][1]).toEqual(post.mock.calls[0][1]);
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});
});

describe('уход с формы новой поставки', () => {
	it('после ввода спрашивает, «Остаться» сохраняет введённое', async () => {
		await openForm();

		await userEvent.type(screen.getByLabelText('Номер поставки'), 'Н-1');
		await userEvent.click(
			within(
				screen.getByRole('navigation', { name: 'Путь к разделу' })
			).getByRole('link', { name: 'Поставки' })
		);

		const dialog = await screen.findByRole('alertdialog', {
			name: 'Уйти без регистрации?',
		});

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Остаться' })
		);

		expect(screen.getByLabelText('Номер поставки')).toHaveValue('Н-1');
	});

	it('без изменений уходит без вопроса', async () => {
		const { router } = await openForm();

		await userEvent.click(screen.getByRole('link', { name: 'Отмена' }));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/lenina/deliveries')
		);
		expect(screen.queryByRole('alertdialog')).toBeNull();
	});

	it('после регистрации вопроса нет', async () => {
		post.mockResolvedValueOnce({ data: registered });
		const { router } = await openForm();

		await fillValid();
		await submit();

		await waitFor(() =>
			expect(router.state.location.pathname).toBe(
				'/points/lenina/deliveries/delivery-4512'
			)
		);
		expect(screen.queryByRole('alertdialog')).toBeNull();
	});
});
