import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { apiClient, ApiError } from '@shared/api';
import { withUnits } from '@shared/lib/test-utils';

import { idempotencyKeyOf, requestsTo } from './test-api';
import { approver, testPoint, testProfile } from './test-router';
import {
	fillWriteOff,
	openTomato,
	stockApi,
	testBatch,
	tomatoBatches,
} from './test-stock';

const post = vi.spyOn(apiClient, 'post');

async function openWriteOff() {
	await userEvent.click(
		await screen.findByRole('button', { name: 'Списать: партия № 111' })
	);

	return screen.findByRole('dialog', { name: 'Списать порчу' });
}

describe('списание порчи', () => {
	it('отправляет граммы, причину, комментарий и ключ, затем показывает итог', async () => {
		post.mockResolvedValueOnce({ data: testBatch({ quantity: 1100 }) });
		const get = stockApi();

		await openTomato();
		const dialog = await openWriteOff();

		expect(
			within(dialog).getByRole('radio', { name: 'Истёк срок годности' })
		).toBeDisabled();

		await fillWriteOff(dialog);
		const before = requestsTo(get, '/stock').length;

		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 500 г') })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
		expect(post.mock.calls[0][0]).toBe(
			'/points/lenina/batches/b-111/write-offs'
		);
		expect(post.mock.calls[0][1]).toEqual({
			quantity: 500,
			reason: 'physical_damage',
			comment: 'Порвался ящик',
		});
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));
		expect(
			await screen.findByText(
				'Списано 500 г · Томаты черри, партия № 111. Осталось 1,1 кг'
			)
		).toBeVisible();
		await waitFor(() =>
			expect(requestsTo(get, '/stock').length).toBeGreaterThan(before)
		);
	});

	it('Enter в поле количества отправляет списание', async () => {
		post.mockResolvedValueOnce({ data: testBatch({ quantity: 1100 }) });
		stockApi();
		await openTomato();
		const dialog = await openWriteOff();

		await fillWriteOff(dialog);
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: /^Количество, кг/ }),
			'{Enter}'
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
		expect(post.mock.calls[0][1]).toEqual({
			quantity: 500,
			reason: 'physical_damage',
			comment: 'Порвался ящик',
		});
	});

	it('без комментария не отправляется', async () => {
		stockApi();
		await openTomato();
		const dialog = await openWriteOff();

		await fillWriteOff(dialog, '');
		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 500 г') })
		);

		expect(
			within(dialog).getByText('Напишите, что случилось с продуктом')
		).toBeVisible();
		expect(post).not.toHaveBeenCalled();
	});

	it('если заказы израсходовали партию, показывает новый остаток и сохраняет ввод', async () => {
		let batches = tomatoBatches;

		post.mockRejectedValueOnce(
			new ApiError('Ошибка проверки', {
				status: 422,
				fieldErrors: {
					quantity: { message: 'превышает количество в партии 300' },
				},
			})
		);
		stockApi({ batches: () => batches });
		await openTomato();
		const dialog = await openWriteOff();

		await fillWriteOff(dialog);
		batches = [testBatch({ quantity: 300 }), ...tomatoBatches.slice(1)];
		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 500 г') })
		);

		expect(
			await within(dialog).findByText(/Сейчас в партии 300 г, было 1,6 кг/)
		).toBeVisible();
		expect(
			within(dialog).getByRole('textbox', { name: /^Комментарий/ })
		).toHaveValue('Порвался ящик');
	});

	it('повтор после сбоя идёт с тем же ключом', async () => {
		post
			.mockRejectedValueOnce(new ApiError('Нет связи с сервером'))
			.mockResolvedValueOnce({ data: testBatch({ quantity: 1100 }) });
		stockApi();
		await openTomato();
		const dialog = await openWriteOff();

		await fillWriteOff(dialog);
		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 500 г') })
		);
		expect(
			await within(dialog).findByText('Нет связи с сервером')
		).toBeVisible();
		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 500 г') })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});
});

describe('исправление остатка', () => {
	it('отправляет изменение от фактического количества и причину', async () => {
		post.mockResolvedValueOnce({ data: testBatch({ quantity: 1400 }) });
		stockApi();
		await openTomato();

		await userEvent.click(
			screen.getByRole('button', { name: 'Ещё действия: партия № 111' })
		);
		await userEvent.click(
			await screen.findByRole('menuitem', { name: 'Исправить остаток' })
		);
		const dialog = await screen.findByRole('dialog', {
			name: 'Исправить остаток',
		});

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: /^Фактически на складе/ }),
			'1,4'
		);
		await userEvent.tab();
		expect(within(dialog).getByText('−200 г')).toBeVisible();
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Пересчёт' })
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Исправить остаток' })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
		expect(post.mock.calls[0][0]).toBe(
			'/points/lenina/batches/b-111/adjustments'
		);
		expect(post.mock.calls[0][1]).toEqual({ delta: -200, reason: 'Пересчёт' });
		expect(
			await screen.findByText(
				'Остаток исправлен · Томаты черри, партия № 111. Сейчас 1,4 кг'
			)
		).toBeVisible();
	});
});

describe('склад только для просмотра', () => {
	it('у приёмщика нет действий с партиями, есть пояснение и история', async () => {
		stockApi();
		await openTomato(testProfile([approver]));

		expect(screen.queryByRole('button', { name: /^Списать/ })).toBeNull();
		expect(
			screen.getByText(
				'Списывать и исправлять остаток могут управляющий и владелец'
			)
		).toBeVisible();
		expect(
			screen.getByRole('button', { name: 'История продукта Томаты черри' })
		).toBeVisible();
	});

	it('на отключённой точке объясняет, почему нельзя списывать', async () => {
		stockApi();
		await openTomato(testProfile([testPoint({ active: false })]));

		expect(screen.queryByRole('button', { name: /^Списать/ })).toBeNull();
		expect(
			screen.getByText(
				'Точка отключена — списывать и исправлять остаток нельзя'
			)
		).toBeVisible();
	});
});
