import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import type { Delivery } from '@entities/delivery';
import { apiClient, ApiError } from '@shared/api';
import { withUnits } from '@shared/lib/test-utils';

import { idempotencyKeyOf, mockGet, paramsOf } from './test-api';
import { testDelivery, testDeliveryItem, testSummary } from './test-deliveries';
import { renderApp, testPoint, testProfile } from './test-router';

const post = vi.spyOn(apiClient, 'post');

const shrimps = testDelivery({
	id: 'delivery-114',
	number: '114',
	status: 'approved',
	approved_at: '2026-10-08T11:05:00Z',
	items: [
		testDeliveryItem({
			id: 'shrimps',
			product_name: 'Креветки 16/20',
			declared_quantity: 6000,
			accepted_quantity: 4000,
			quarantined_quantity: 2000,
			quarantine_reason: 'storage_violation',
			comment: '−12 °C вместо −18 °C',
			quarantined_at: '2026-10-08T11:05:00Z',
		}),
	],
});

const mozzarella = testDelivery({
	id: 'delivery-116',
	number: '116',
	status: 'approved',
	approved_at: '2026-10-09T12:40:00Z',
	items: [
		testDeliveryItem({
			id: 'mozzarella',
			product_name: 'Моцарелла для пиццы',
			declared_quantity: 5000,
			accepted_quantity: 3500,
			quarantined_quantity: 1500,
			quarantine_reason: 'physical_damage',
			quarantined_at: '2026-10-09T12:40:00Z',
		}),
	],
});

/** Поставка после решения: карантин позиции закрыт. */
function resolved(delivery: Delivery, decidedAt: string, name = 'Анна Котова') {
	return {
		...delivery,
		items: delivery.items.map((item) => ({
			...item,
			accepted_quantity: item.accepted_quantity + item.quarantined_quantity,
			quarantined_quantity: 0,
			quarantined_at: undefined,
			quarantine_resolutions: [
				{
					decided_at: decidedAt,
					decided_by: { type: 'user' as const, id: 'user', name },
					accepted_quantity: item.quarantined_quantity,
					rejected_quantity: 0,
				},
			],
		})),
	};
}

/** Ответы API: список с карантином, сводка и карточка; `detail` подменяет карточку поставки. */
function quarantineApi({
	open = [mozzarella, shrimps],
	detail = {},
}: { open?: Delivery[]; detail?: Record<string, Delivery> } = {}) {
	return mockGet((url, params) => {
		if (url.endsWith('/deliveries/summary')) {
			return testSummary({ open_quarantine_items: open.length });
		}

		if (url.endsWith('/deliveries')) {
			const items = params.quarantine === 'open' ? open : [];

			return { items, total: items.length };
		}

		const found = Object.entries(detail).find(([id]) =>
			url.endsWith(`/deliveries/${id}`)
		);

		if (!found) {
			throw new ApiError('Поставка не найдена', { status: 404 });
		}

		return found[1];
	});
}

async function openPanel(options?: Parameters<typeof renderApp>[1]) {
	const view = await renderApp(
		'/points/lenina/deliveries?panel=quarantine',
		options
	);
	const panel = await screen.findByRole('dialog', {
		name: /Открытый карантин/,
	});

	await within(panel).findByRole('list', { name: 'Позиции в карантине' });

	return { ...view, panel };
}

function card(name: string) {
	return screen.getByRole('listitem', { name });
}

describe('открытый карантин', () => {
	it('ссылка с открытой панелью показывает позиции открытого карантина, первыми дольше ждущие', async () => {
		const get = quarantineApi();
		const { panel } = await openPanel();

		expect(
			paramsOf(get, '/deliveries').some(
				(params) => params.quarantine === 'open'
			)
		).toBe(true);
		expect(
			within(panel)
				.getAllByRole('listitem')
				.map((item) => item.getAttribute('aria-label'))
		).toEqual(['Креветки 16/20', 'Моцарелла для пиццы']);
		expect(
			within(card('Креветки 16/20')).getByText(
				'Нарушен температурный режим — «−12 °C вместо −18 °C»'
			)
		).toBeInTheDocument();
	});

	it('кнопка в шапке открывает панель, «Назад» в браузере закрывает её', async () => {
		quarantineApi();
		const { router } = await renderApp('/points/lenina/deliveries');

		await userEvent.click(
			await screen.findByRole('button', { name: /Открытый карантин/ })
		);

		expect(
			await screen.findByRole('dialog', { name: /Открытый карантин/ })
		).toBeInTheDocument();
		expect(router.state.location.search).toEqual({ panel: 'quarantine' });

		act(() => router.history.back());

		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	});

	it('без позиций в карантине панель говорит, что карантина нет', async () => {
		quarantineApi({ open: [] });
		await renderApp('/points/lenina/deliveries?panel=quarantine');

		expect(
			await screen.findByText('Открытого карантина нет')
		).toBeInTheDocument();
	});
});

describe('решение по карантину', () => {
	it('«Принять на склад» отправляет решение с ключом, позиция уходит из панели с сообщением', async () => {
		quarantineApi();
		await openPanel();

		post.mockResolvedValueOnce({
			data: resolved(shrimps, '2026-10-10T14:39:00Z'),
		});
		await userEvent.click(
			within(card('Креветки 16/20')).getByRole('button', {
				name: withUnits('Принять 2,0 кг на склад'),
			})
		);

		expect(
			await screen.findByText('Решение по «Креветки 16/20» сохранено')
		).toBeInTheDocument();
		expect(
			screen.queryByRole('listitem', { name: 'Креветки 16/20' })
		).toBeNull();
		expect(post.mock.calls[0][0]).toBe(
			'/points/lenina/deliveries/delivery-114/items/shrimps/quarantine-resolution'
		);
		expect(post.mock.calls[0][1]).toEqual({
			accepted_quantity: 2000,
			rejected_quantity: 0,
			reject_reason: undefined,
			comment: undefined,
		});
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));
	});

	it('часть отклонить: нужны причина и комментарий, действие называется «Принять решение»', async () => {
		quarantineApi();
		await openPanel();
		const mozzarellaCard = card('Моцарелла для пиццы');

		const accept = within(mozzarellaCard).getByLabelText(
			'Моцарелла для пиццы: принять, кг'
		);

		await userEvent.clear(accept);
		await userEvent.type(accept, '1');
		const reject = within(mozzarellaCard).getByLabelText(
			'Моцарелла для пиццы: отклонить, кг'
		);

		await userEvent.clear(reject);
		await userEvent.type(reject, '0,5');
		await userEvent.tab();

		const action = within(mozzarellaCard).getByRole('button', {
			name: 'Принять решение',
		});

		expect(action).toBeDisabled();
		expect(
			within(mozzarellaCard).getByText('Напишите комментарий — он обязателен')
		).toBeInTheDocument();

		await userEvent.click(
			within(mozzarellaCard).getByRole('combobox', {
				name: 'Причина отклонения',
			})
		);
		await userEvent.click(
			screen.getByRole('option', { name: 'Повреждена упаковка' })
		);
		await userEvent.type(
			within(mozzarellaCard).getByRole('textbox', { name: 'Комментарий' }),
			'один блок вскрыт'
		);

		expect(action).toBeEnabled();
		expect(
			within(mozzarellaCard).getByText(
				'Принять 1,0 кг — новая партия; 0,5 кг будут окончательно отклонены'
			)
		).toBeInTheDocument();
	});

	it('карантин уже решил другой: сообщение с именем и временем, позиции в панели нет', async () => {
		const decidedAt = new Date(2026, 9, 10, 14, 39).toISOString();

		quarantineApi({
			detail: {
				'delivery-114': resolved(shrimps, decidedAt, 'Илья Смирнов'),
			},
		});
		await openPanel();

		post.mockRejectedValueOnce(
			new ApiError('у позиции нет открытого карантина', {
				status: 409,
				type: 'quarantine-closed',
			})
		);
		await userEvent.click(
			within(card('Креветки 16/20')).getByRole('button', {
				name: withUnits('Принять 2,0 кг на склад'),
			})
		);

		expect(
			await screen.findByText(
				'Карантин по «Креветки 16/20» уже решён: Илья Смирнов, 14:39'
			)
		).toBeInTheDocument();
		expect(
			screen.queryByRole('listitem', { name: 'Креветки 16/20' })
		).toBeNull();
	});

	it('отказ сервера по сроку делает «Принять» недоступным и предлагает отклонить всё', async () => {
		quarantineApi();
		await openPanel();

		post.mockRejectedValueOnce(
			new ApiError('Ошибка валидации', {
				status: 422,
				fieldErrors: {
					accepted_quantity: {
						message: 'позиция просрочена и не может быть принята',
						code: 'expired',
					},
				},
			})
		);
		await userEvent.click(
			within(card('Креветки 16/20')).getByRole('button', {
				name: withUnits('Принять 2,0 кг на склад'),
			})
		);

		const shrimpsCard = card('Креветки 16/20');

		expect(
			await within(shrimpsCard).findByRole('button', {
				name: withUnits('Отклонить 2,0 кг окончательно'),
			})
		).toBeEnabled();
		expect(
			within(shrimpsCard).getByLabelText('Креветки 16/20: принять, кг')
		).toBeDisabled();
		expect(
			within(shrimpsCard).getByText('проставлена автоматически')
		).toBeInTheDocument();
	});

	it('«Повторить» после обрыва отправляет тот же ключ', async () => {
		quarantineApi();
		await openPanel();
		const offline = new ApiError(
			'Нет связи с сервером. Проверьте подключение и повторите'
		);

		post.mockRejectedValueOnce(offline).mockResolvedValueOnce({
			data: resolved(shrimps, '2026-10-10T14:39:00Z'),
		});
		await userEvent.click(
			within(card('Креветки 16/20')).getByRole('button', {
				name: withUnits('Принять 2,0 кг на склад'),
			})
		);
		await userEvent.click(
			await within(card('Креветки 16/20')).findByRole('button', {
				name: 'Повторить',
			})
		);

		await screen.findByText('Решение по «Креветки 16/20» сохранено');
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});

	it('на отключённой точке позиции видны без полей и действий', async () => {
		quarantineApi();
		const { panel } = await openPanel({
			profile: testProfile([testPoint({ active: false })]),
		});

		expect(
			within(panel).getByText('Точка отключена — решить карантин нельзя')
		).toBeInTheDocument();
		expect(within(panel).getByText('Креветки 16/20')).toBeInTheDocument();
		expect(within(panel).queryByRole('spinbutton')).toBeNull();
		expect(within(panel).queryByRole('textbox')).toBeNull();
		expect(within(panel).queryByRole('button', { name: /Принять/ })).toBeNull();
	});
});
