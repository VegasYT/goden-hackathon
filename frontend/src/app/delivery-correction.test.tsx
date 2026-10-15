import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import type { Delivery } from '@entities/delivery';
import { apiClient, ApiError } from '@shared/api';
import { retype } from '@shared/lib/test-utils';

import { idempotencyKeyOf } from './test-api';
import {
	deliveriesApi,
	testDelivery,
	testDeliveryItem,
} from './test-deliveries';
import { renderApp } from './test-router';

const post = vi.spyOn(apiClient, 'post');

const salmon = testDeliveryItem({
	id: 'salmon',
	product_name: 'Лосось охлаждённый',
	declared_quantity: 12000,
	accepted_quantity: 12000,
	in_stock_quantity: 6500,
	batch_ids: ['b1'],
	approved: {
		accepted_quantity: 12000,
		rejected_quantity: 0,
		quarantined_quantity: 0,
	},
});
const flour = testDeliveryItem({
	id: 'flour',
	product_name: 'Мука пшеничная в/с',
	declared_quantity: 25000,
	accepted_quantity: 25000,
	in_stock_quantity: 25000,
	batch_ids: ['b2'],
	approved: {
		accepted_quantity: 25000,
		rejected_quantity: 0,
		quarantined_quantity: 0,
	},
});
const spinach = testDeliveryItem({
	id: 'spinach',
	product_name: 'Шпинат свежий',
	declared_quantity: 3000,
	rejected_quantity: 3000,
	expired: true,
	expires_at: '2026-10-09T22:00:00Z',
	reject_reason: 'expired',
	approved: {
		accepted_quantity: 0,
		rejected_quantity: 3000,
		quarantined_quantity: 0,
	},
});

const delivery = testDelivery({
	status: 'approved',
	approved_at: '2026-10-11T14:41:00Z',
	approved_by: { type: 'user', id: 'anna', name: 'Анна Котова' },
	items: [salmon, flour, spinach],
});

/** Поставка после исправления муки: 20 кг принято, 5 кг отклонено. */
const corrected: Delivery = {
	...delivery,
	corrected_at: '2026-10-12T10:05:00Z',
	items: [
		salmon,
		{
			...flour,
			accepted_quantity: 20000,
			rejected_quantity: 5000,
			in_stock_quantity: 20000,
		},
		spinach,
	],
};

async function openCorrection() {
	deliveriesApi({ list: [delivery] });
	const view = await renderApp(
		'/points/lenina/deliveries/delivery-118/correction'
	);

	await screen.findByRole('heading', {
		level: 1,
		name: 'Исправление приёмки № 118',
	});

	return view;
}

const setQuantity = (label: string, value: string) =>
	retype(screen.getByLabelText(label), value);

async function correctFlour() {
	await setQuantity('Мука пшеничная в/с: принять, кг', '20');
	await setQuantity('Мука пшеничная в/с: отклонить, кг', '5');
	await userEvent.type(
		screen.getByRole('textbox', { name: 'Причина исправления' }),
		'Пересчёт'
	);
}

function saveButton() {
	return within(
		screen.getByRole('region', { name: 'Итог исправления' })
	).getByRole('button', { name: 'Сохранить исправление' });
}

async function openConfirm() {
	await userEvent.click(saveButton());

	return screen.findByRole('alertdialog', {
		name: 'Сохранить исправление приёмки № 118?',
	});
}

describe('исправление приёмки', () => {
	it('уменьшение сверх остатка показывает предел и не даёт сохранить, у просроченной позиции принять нельзя', async () => {
		await openCorrection();

		expect(screen.getByLabelText('Шпинат свежий: принять, кг')).toBeDisabled();

		await setQuantity('Лосось охлаждённый: принять, кг', '4');

		expect(
			screen.getByText(
				'Принятое можно уменьшить не больше чем на 6,5 кг: остальное уже израсходовано'
			)
		).toBeInTheDocument();
		expect(saveButton()).toBeDisabled();

		await userEvent.click(screen.getByRole('button', { name: /С ошибкой/ }));

		expect(
			within(screen.getByRole('table', { name: 'Позиции поставки' }))
				.getAllByRole('row')
				.slice(1)
				.map((row) => within(row).getAllByRole('cell')[0].textContent)
		).toEqual([expect.stringContaining('Лосось охлаждённый')]);
	});

	it('под названием позиции — её категория', async () => {
		await openCorrection();

		const product = within(
			within(screen.getByRole('table', { name: 'Позиции поставки' })).getByRole(
				'cell',
				{ name: /Лосось охлаждённый/ }
			)
		);

		expect(product.getByText('Лосось охлаждённый')).toBeInTheDocument();
		expect(product.getByText('Молочные продукты')).toBeInTheDocument();
	});

	it('без изменений и без причины сохранить нельзя', async () => {
		await openCorrection();

		expect(saveButton()).toBeDisabled();

		await setQuantity('Мука пшеничная в/с: принять, кг', '20');
		await setQuantity('Мука пшеничная в/с: отклонить, кг', '5');

		expect(screen.getByText('было 25,0')).toBeInTheDocument();
		expect(screen.getByText('−5,0 кг со склада')).toBeInTheDocument();
		expect(saveButton()).toBeDisabled();
	});

	it('сохранение отправляет только изменённые позиции с причиной и ключом, затем открывает снимок', async () => {
		const { router } = await openCorrection();

		await correctFlour();

		const dialog = await openConfirm();

		expect(
			within(dialog).getByText('Со склада будет снято 5,0 кг')
		).toBeInTheDocument();

		post.mockResolvedValueOnce({ data: corrected });
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Сохранить исправление' })
		);

		expect(
			await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' })
		).toBeInTheDocument();
		expect(router.state.location.pathname).toBe(
			'/points/lenina/deliveries/delivery-118'
		);
		expect(screen.getByText('исправлена')).toBeInTheDocument();
		expect(screen.getByLabelText('было 25,0')).toBeInTheDocument();
		expect(post.mock.calls[0][0]).toBe(
			'/points/lenina/deliveries/delivery-118/corrections'
		);
		expect(post.mock.calls[0][1]).toEqual({
			reason: 'Пересчёт',
			items: [
				{
					item_id: 'flour',
					accepted_quantity: 20000,
					rejected_quantity: 5000,
					quarantined_quantity: 0,
				},
			],
		});
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));
	});

	it('отказ сервера по остатку закрывает диалог и показывает предел сервера', async () => {
		await openCorrection();
		await correctFlour();

		post.mockRejectedValueOnce(
			new ApiError('Ошибка валидации', {
				status: 422,
				fieldErrors: {
					'items[flour].accepted_quantity': {
						message: 'можно уменьшить не более чем на 3000',
						code: 'stock-consumed',
						limit: 3000,
					},
				},
			})
		);
		await userEvent.click(
			within(await openConfirm()).getByRole('button', {
				name: 'Сохранить исправление',
			})
		);

		await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
		expect(
			screen.getByText(
				'Принятое можно уменьшить не больше чем на 3,0 кг: остальное уже израсходовано'
			)
		).toBeInTheDocument();
		expect(saveButton()).toBeDisabled();
	});

	it('«Повторить» после обрыва отправляет тот же ключ, «Вернуться к исправлению» ничего не отправляет', async () => {
		await openCorrection();
		await correctFlour();

		const back = await openConfirm();

		await userEvent.click(
			within(back).getByRole('button', { name: 'Вернуться к исправлению' })
		);
		await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
		expect(post).not.toHaveBeenCalled();

		post
			.mockRejectedValueOnce(
				new ApiError('Нет связи с сервером. Проверьте подключение и повторите')
			)
			.mockResolvedValueOnce({ data: corrected });

		const dialog = await openConfirm();

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Сохранить исправление' })
		);
		await userEvent.click(
			await within(dialog).findByRole('button', { name: 'Повторить' })
		);

		await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' });
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});
});

describe('уход с несохранённым исправлением', () => {
	it('после изменения «Отменить» спрашивает: «Остаться» оставляет значения, «Уйти» открывает снимок', async () => {
		await openCorrection();
		await setQuantity('Мука пшеничная в/с: принять, кг', '20');

		await userEvent.click(screen.getByRole('link', { name: 'Отменить' }));

		const dialog = await screen.findByRole('alertdialog', {
			name: 'Уйти без сохранения исправления?',
		});

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Остаться' })
		);
		await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
		expect(
			screen.getByLabelText('Мука пшеничная в/с: принять, кг')
		).toHaveValue('20,0');

		await userEvent.click(screen.getByRole('link', { name: 'Отменить' }));
		await userEvent.click(
			within(
				await screen.findByRole('alertdialog', {
					name: 'Уйти без сохранения исправления?',
				})
			).getByRole('button', { name: 'Уйти' })
		);

		expect(
			await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' })
		).toBeInTheDocument();
	});

	it('без изменений уходит без вопроса', async () => {
		await openCorrection();

		await userEvent.click(screen.getByRole('link', { name: 'Отменить' }));

		expect(
			await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' })
		).toBeInTheDocument();
		expect(screen.queryByRole('alertdialog')).toBeNull();
	});
});
