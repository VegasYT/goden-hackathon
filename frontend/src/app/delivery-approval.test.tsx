import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { apiClient, ApiError } from '@shared/api';
import { retype } from '@shared/lib/test-utils';

import { idempotencyKeyOf } from './test-api';
import {
	approvedDelivery,
	deliveriesApi,
	testDelivery,
	testDeliveryItem,
} from './test-deliveries';
import { renderApp } from './test-router';

const eggs = testDeliveryItem({
	id: 'eggs',
	product_id: 'product-eggs',
	product_name: 'Яйца куриные С0',
	unit: 'pcs',
	declared_quantity: 360,
});

const post = vi.spyOn(apiClient, 'post');

async function openDelivery(
	delivery = testDelivery({ items: [testDeliveryItem(), eggs] })
) {
	deliveriesApi({ list: [delivery] });
	const view = await renderApp(`/points/lenina/deliveries/${delivery.id}`);

	await screen.findByRole('heading', {
		level: 1,
		name: 'Приёмка поставки № 118',
	});

	return { ...view, delivery };
}

const setQuantity = (label: string, value: string) =>
	retype(screen.getByLabelText(label), value);

async function openConfirm() {
	await userEvent.click(
		within(screen.getByRole('region', { name: 'Итог приёмки' })).getByRole(
			'button',
			{
				name: 'Подтвердить приёмку',
			}
		)
	);

	return screen.findByRole('alertdialog', {
		name: /Подтвердить приёмку поставки № 118/,
	});
}

/** Открывает диалог подтверждения и подтверждает приёмку. */
async function submitApproval() {
	const dialog = await openConfirm();

	await userEvent.click(
		within(dialog).getByRole('button', { name: 'Подтвердить приёмку' })
	);

	return dialog;
}

async function fillRejection(reason: string, comment: string) {
	await userEvent.click(
		screen.getByRole('combobox', { name: 'Причина отклонения' })
	);
	await userEvent.click(screen.getByRole('option', { name: reason }));
	await userEvent.type(
		screen.getByRole('textbox', { name: 'Комментарий' }),
		comment
	);
}

describe('приёмка поставки', () => {
	it('поставка без проблем подтверждается сразу: целые количества, ключ и снимок после ответа', async () => {
		const { delivery } = await openDelivery();

		post.mockResolvedValueOnce({ data: approvedDelivery(delivery) });
		await submitApproval();

		expect(
			await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' })
		).toBeInTheDocument();
		expect(post).toHaveBeenCalledTimes(1);
		expect(post.mock.calls[0][0]).toBe(
			'/points/lenina/deliveries/delivery-118/approval'
		);
		expect(post.mock.calls[0][1]).toEqual({
			items: [
				expect.objectContaining({
					item_id: 'parmesan',
					accepted_quantity: 4000,
					rejected_quantity: 0,
				}),
				expect.objectContaining({ item_id: 'eggs', accepted_quantity: 360 }),
			],
		});
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));
	});

	it('отклонение без комментария держит подтверждение неактивным, пока комментарий не написан', async () => {
		await openDelivery();

		await setQuantity('Пармезан 24 мес.: принять, кг', '3');
		await setQuantity('Пармезан 24 мес.: отклонить, кг', '1');

		const confirm = within(
			screen.getByRole('region', { name: 'Итог приёмки' })
		).getByRole('button', { name: 'Подтвердить приёмку' });

		expect(confirm).toBeDisabled();
		expect(
			screen.getByText('Напишите комментарий — он обязателен')
		).toBeInTheDocument();

		await fillRejection('Повреждена упаковка', 'Надорван вакуум');

		expect(confirm).toBeEnabled();
	});

	it('пустые причина и комментарий краснеют только после ухода с поля, подсказка видна сразу', async () => {
		await openDelivery();

		await setQuantity('Пармезан 24 мес.: принять, кг', '3');
		await setQuantity('Пармезан 24 мес.: отклонить, кг', '1');

		const reason = screen.getByRole('combobox', { name: 'Причина отклонения' });
		const comment = screen.getByRole('textbox', { name: 'Комментарий' });

		expect(reason).not.toBeInvalid();
		expect(comment).not.toBeInvalid();
		expect(screen.getByText('Выберите причину')).toBeInTheDocument();

		await userEvent.click(reason);
		await userEvent.keyboard('{Escape}');
		await userEvent.tab();

		expect(reason).toBeInvalid();
		expect(comment).not.toBeInvalid();

		await userEvent.click(comment);
		await userEvent.tab();

		expect(comment).toBeInvalid();
	});

	it('фильтр «Требуют решения» оставляет только нерешённую позицию', async () => {
		await openDelivery();

		await setQuantity('Пармезан 24 мес.: принять, кг', '');
		await userEvent.click(
			screen.getByRole('button', { name: /Требуют решения/ })
		);

		const table = screen.getByRole('table', { name: 'Позиции поставки' });

		expect(within(table).getByText('Пармезан 24 мес.')).toBeInTheDocument();
		expect(within(table).queryByText('Яйца куриные С0')).toBeNull();
		expect(
			within(table).getByText(/Распределите 4,0.кг между полями/)
		).toBeInTheDocument();
	});

	it('«Вернуться к приёмке» ничего не отправляет', async () => {
		await openDelivery();

		const dialog = await openConfirm();

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Вернуться к приёмке' })
		);

		expect(post).not.toHaveBeenCalled();
	});

	it('«Повторить» после обрыва отправляет тот же ключ, изменённые решения — новый', async () => {
		const { delivery } = await openDelivery();
		const offline = new ApiError(
			'Нет связи с сервером. Проверьте подключение и повторите'
		);

		post.mockRejectedValueOnce(offline).mockRejectedValueOnce(offline);

		const dialog = await submitApproval();

		await userEvent.click(
			await within(dialog).findByRole('button', { name: 'Повторить' })
		);
		await within(dialog).findByText(/Не удалось подтвердить приёмку/);

		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Вернуться к приёмке' })
		);
		await setQuantity('Яйца куриные С0: принять, шт', '300');
		await setQuantity('Яйца куриные С0: отклонить, шт', '60');
		await fillRejection('Повреждено при перевозке', 'Разбиты');

		post.mockResolvedValueOnce({ data: approvedDelivery(delivery) });
		await submitApproval();

		await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' });
		expect(idempotencyKeyOf(post.mock.calls[2])).not.toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});

	it('отказ сервера по сроку обнуляет принятое у позиции и закрывает диалог', async () => {
		await openDelivery();

		post.mockRejectedValueOnce(
			new ApiError('Ошибка валидации', {
				status: 422,
				fieldErrors: {
					'items[parmesan].accepted_quantity': {
						message: 'позиция просрочена и не может быть принята',
						code: 'expired',
					},
				},
			})
		);

		await submitApproval();

		await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
		expect(
			screen.getByLabelText('Пармезан 24 мес.: принять, кг')
		).toBeDisabled();
		expect(screen.getByText('было 4,0')).toBeInTheDocument();
		expect(screen.getByText(/пока заполнялась приёмка/)).toBeInTheDocument();
	});
});

describe('срок истекает при открытой форме', () => {
	beforeEach(() => {
		vi.useFakeTimers({ shouldAdvanceTime: true });
		vi.setSystemTime(new Date(2026, 9, 12, 14, 32, 30));
	});

	it('через минуту позиция становится просроченной с прежним значением', async () => {
		await openDelivery(
			testDelivery({
				items: [
					testDeliveryItem({
						expires_at: new Date(2026, 9, 12, 14, 33).toISOString(),
					}),
				],
			})
		);

		expect(
			screen.getByLabelText('Пармезан 24 мес.: принять, кг')
		).toBeEnabled();

		await act(() => vi.advanceTimersByTimeAsync(60_000));

		expect(
			screen.getByLabelText('Пармезан 24 мес.: принять, кг')
		).toBeDisabled();
		expect(screen.getByText('было 4,0')).toBeInTheDocument();
		expect(screen.getByText(/пока заполнялась приёмка/)).toBeInTheDocument();
	});
});

describe('уход с несохранёнными решениями', () => {
	it('после изменения переход спрашивает: «Остаться» оставляет решения, «Уйти» уводит', async () => {
		const { router } = await openDelivery();

		await setQuantity('Пармезан 24 мес.: принять, кг', '3');
		await userEvent.click(screen.getByRole('link', { name: 'Склад' }));

		let dialog = await screen.findByRole('alertdialog', {
			name: 'Уйти без подтверждения приёмки?',
		});

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Остаться' })
		);
		await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());

		expect(router.state.location.pathname).toBe(
			'/points/lenina/deliveries/delivery-118'
		);
		expect(screen.getByLabelText('Пармезан 24 мес.: принять, кг')).toHaveValue(
			'3,0'
		);

		await userEvent.click(screen.getByRole('link', { name: 'Склад' }));
		dialog = await screen.findByRole('alertdialog', {
			name: 'Уйти без подтверждения приёмки?',
		});
		await userEvent.click(within(dialog).getByRole('button', { name: 'Уйти' }));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/lenina/stock')
		);
	});

	it('без изменений уходит без вопроса', async () => {
		const { router } = await openDelivery();

		await userEvent.click(screen.getByRole('link', { name: 'Склад' }));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/lenina/stock')
		);
		expect(screen.queryByRole('alertdialog')).toBeNull();
	});

	it('после успешной приёмки уходит без вопроса', async () => {
		const { router, delivery } = await openDelivery();

		await setQuantity('Яйца куриные С0: принять, шт', '360');
		post.mockResolvedValueOnce({ data: approvedDelivery(delivery) });
		await submitApproval();
		await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' });

		await userEvent.click(screen.getByRole('link', { name: 'Склад' }));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/lenina/stock')
		);
		expect(screen.queryByRole('alertdialog', { name: /Уйти/ })).toBeNull();
	});
});
