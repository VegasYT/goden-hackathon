import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import type { Delivery } from '@entities/delivery';
import { apiClient, ApiError } from '@shared/api';

import { mockGet, requestsTo } from './test-api';
import {
	approvedDelivery,
	deliveriesApi,
	testDelivery,
	testDeliveryItem,
} from './test-deliveries';
import { renderApp, testPoint, testProfile } from './test-router';

const approvedAt = new Date(2026, 9, 11, 14, 41).toISOString();

describe('снимок принятой поставки', () => {
	it('показывает решённый карантин, исправленное значение и старую причину как есть', async () => {
		deliveriesApi({
			list: [
				testDelivery({
					status: 'approved',
					approved_at: approvedAt,
					approved_by: { type: 'service', id: 'goden-generator' },
					corrected_at: new Date(2026, 9, 12, 10, 5).toISOString(),
					items: [
						testDeliveryItem({
							id: 'mascarpone',
							product_name: 'Маскарпоне',
							declared_quantity: 6000,
							accepted_quantity: 5500,
							rejected_quantity: 500,
							quarantine_reason: 'storage_violation',
							comment: '+9 °C в двух коробках',
							approved: {
								accepted_quantity: 4000,
								rejected_quantity: 0,
								quarantined_quantity: 2000,
							},
							quarantine_resolutions: [
								{
									decided_at: new Date(2026, 9, 12, 12, 30).toISOString(),
									decided_by: { type: 'user', id: 'anna', name: 'Анна Котова' },
									accepted_quantity: 1500,
									rejected_quantity: 500,
								},
							],
							batch_ids: ['b1', 'b2'],
						}),
						testDeliveryItem({
							id: 'flour',
							product_name: 'Мука пшеничная в/с',
							declared_quantity: 25000,
							accepted_quantity: 20000,
							rejected_quantity: 5000,
							reject_reason: 'пересчёт после разгрузки',
							comment: 'муки 4 мешка, а не 5',
							approved: {
								accepted_quantity: 25000,
								rejected_quantity: 0,
								quarantined_quantity: 0,
							},
							batch_ids: ['b3'],
						}),
					],
				}),
			],
		});
		await renderApp('/points/lenina/deliveries/delivery-118');

		expect(
			await screen.findByRole('heading', { level: 1, name: 'Поставка № 118' })
		).toBeInTheDocument();
		expect(screen.getByText(/приёмка: Генератор/)).toBeInTheDocument();
		expect(screen.getByText('исправлена')).toBeInTheDocument();

		const table = screen.getByRole('table', { name: 'Позиции поставки' });
		const mascarpone = within(table).getByRole('row', { name: /Маскарпоне/ });
		const flour = within(table).getByRole('row', {
			name: /Мука пшеничная в\/с/,
		});

		expect(
			within(mascarpone).getByText('решён 12.10: 1,5 принято, 0,5 отклонено')
		).toBeInTheDocument();
		// Решение по карантину — не исправление: у маскарпоне ничего не зачёркнуто.
		expect(within(mascarpone).queryByRole('deletion')).toBeNull();
		expect(within(flour).getByLabelText('было 25,0')).toBeInTheDocument();
		expect(
			within(flour).getByText('пересчёт после разгрузки')
		).toBeInTheDocument();
	});
});

describe('карточка поставки', () => {
	it('открытие после наведения на ссылку не запрашивает поставку второй раз', async () => {
		vi.useFakeTimers({ toFake: ['Date'] });
		const get = deliveriesApi();
		const card = {
			to: '/points/$pointId/deliveries/$deliveryId',
			params: { pointId: 'lenina', deliveryId: 'delivery-118' },
		} as const;
		const { router } = await renderApp('/points/lenina/deliveries');

		// Наведение загружает карточку заранее, а щелчок приходит через пару секунд.
		await act(() => router.preloadRoute(card));
		vi.setSystemTime(Date.now() + 2000);
		await act(() => router.navigate(card));

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Приёмка поставки № 118',
			})
		).toBeInTheDocument();
		expect(requestsTo(get, '/deliveries/delivery-118')).toHaveLength(1);
	});

	it('поставки нет на точке: «Поставка не найдена» и переход к списку', async () => {
		deliveriesApi({ list: [] });
		await renderApp('/points/lenina/deliveries/unknown');

		expect(
			await screen.findByRole('heading', { name: 'Поставка не найдена' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('link', { name: 'К списку поставок' })
		).toHaveAttribute('href', '/points/lenina/deliveries');
	});

	it('поставку уже принял другой: снимок его приёмки с именем и временем', async () => {
		const pending = testDelivery();
		let current = pending;

		mockGet(() => current);
		vi.spyOn(apiClient, 'post').mockImplementationOnce(async () => {
			current = approvedDelivery(pending, {
				type: 'user',
				id: 'ilya',
				name: 'Илья Смирнов',
			});
			throw new ApiError('поставка уже принята', {
				status: 409,
				type: 'delivery-already-approved',
			});
		});
		await renderApp('/points/lenina/deliveries/delivery-118');

		await userEvent.click(
			within(
				await screen.findByRole('region', { name: 'Итог приёмки' })
			).getByRole('button', { name: 'Подтвердить приёмку' })
		);
		await userEvent.click(
			within(await screen.findByRole('alertdialog')).getByRole('button', {
				name: 'Подтвердить приёмку',
			})
		);

		expect(
			await screen.findByText(
				'Поставка № 118 уже принята: Илья Смирнов, 14:39. Ваши решения не сохранены.'
			)
		).toBeInTheDocument();
		await waitFor(() =>
			expect(
				screen.getByRole('heading', { level: 1, name: 'Поставка № 118' })
			).toBeInTheDocument()
		);
	});

	it('на отключённой точке поставка только для просмотра', async () => {
		deliveriesApi();
		await renderApp('/points/lenina/deliveries/delivery-118', {
			profile: testProfile([testPoint({ active: false })]),
		});

		expect(
			await screen.findByText('Точка отключена — принять поставку нельзя')
		).toBeInTheDocument();
		expect(screen.queryByRole('spinbutton')).toBeNull();
		expect(screen.queryByRole('textbox', { name: /принять/ })).toBeNull();
		expect(
			screen.queryByRole('button', { name: 'Подтвердить приёмку' })
		).toBeNull();
		expect(screen.getByText('Пармезан 24 мес.')).toBeInTheDocument();
	});
});

describe('после приёмки', () => {
	const quarantined = approvedDelivery(
		testDelivery({
			items: [
				testDeliveryItem({
					id: 'mascarpone',
					product_name: 'Маскарпоне',
					declared_quantity: 6000,
				}),
			],
		})
	);
	const withQuarantine: Delivery = {
		...quarantined,
		items: quarantined.items.map((item) => ({
			...item,
			accepted_quantity: 4000,
			quarantined_quantity: 2000,
			quarantine_reason: 'storage_violation',
			comment: '+9 °C',
		})),
	};

	it('адрес исправления ожидающей поставки открывает форму приёмки', async () => {
		deliveriesApi();
		const { router } = await renderApp(
			'/points/lenina/deliveries/delivery-118/correction'
		);

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Приёмка поставки № 118',
			})
		).toBeInTheDocument();
		expect(router.state.location.pathname).toBe(
			'/points/lenina/deliveries/delivery-118'
		);
	});

	it('у открытого карантина на снимке ссылка открывает список с панелью карантина', async () => {
		deliveriesApi({ list: [withQuarantine] });
		const { router } = await renderApp(
			'/points/lenina/deliveries/delivery-118'
		);

		await userEvent.click(
			await screen.findByRole('link', { name: 'Открыть карантин' })
		);

		expect(
			await screen.findByRole('dialog', { name: /Открытый карантин/ })
		).toBeInTheDocument();
		expect(router.state.location.pathname).toBe('/points/lenina/deliveries');
	});

	it('на отключённой точке у снимка есть история и нет исправления', async () => {
		deliveriesApi({ list: [withQuarantine] });
		await renderApp('/points/lenina/deliveries/delivery-118', {
			profile: testProfile([testPoint({ active: false })]),
		});

		expect(
			await screen.findByRole('button', { name: 'История событий' })
		).toBeInTheDocument();
		expect(
			screen.queryByRole('link', { name: 'Исправить приёмку' })
		).toBeNull();
	});

	it('история называет авторов событий, показывает склеенное решение по карантину и изменения исправления, «Назад» закрывает её', async () => {
		const flour = testDeliveryItem({
			id: 'flour',
			product_name: 'Мука пшеничная в/с',
			declared_quantity: 25000,
			accepted_quantity: 25000,
		});
		const approvedState: Delivery = {
			...withQuarantine,
			items: [...withQuarantine.items, flour],
		};
		const at = '2026-10-12T12:30:00Z';
		const withItems = (
			mascarpone: Partial<typeof flour>,
			flourAccepted = 25000
		) => ({
			...approvedState,
			items: [
				{ ...approvedState.items[0], ...mascarpone },
				{
					...flour,
					accepted_quantity: flourAccepted,
					rejected_quantity: 25000 - flourAccepted,
				},
			],
		});
		const anna = { type: 'user' as const, id: 'anna', name: 'Анна Котова' };

		mockGet((url) => {
			if (url.endsWith('/history')) {
				return {
					items: [
						{
							type: 'registered',
							occurred_at: '2026-10-11T14:21:00Z',
							actor: { type: 'service', id: 'goden-generator' },
							delivery: approvedState,
						},
						{
							type: 'approved',
							occurred_at: '2026-10-11T14:41:00Z',
							actor: anna,
							delivery: approvedState,
						},
						{
							type: 'corrected',
							occurred_at: '2026-10-12T10:05:00Z',
							actor: anna,
							reason: 'Пересчитали после разгрузки',
							delivery: withItems({}, 20000),
						},
						{
							type: 'quarantine_accepted',
							occurred_at: at,
							actor: anna,
							delivery: withItems(
								{ accepted_quantity: 5500, quarantined_quantity: 500 },
								20000
							),
						},
						{
							type: 'quarantine_rejected',
							occurred_at: at,
							actor: anna,
							reject_reason: 'storage_violation',
							delivery: withItems(
								{
									accepted_quantity: 5500,
									rejected_quantity: 500,
									quarantined_quantity: 0,
								},
								20000
							),
						},
					],
				};
			}

			return approvedState;
		});
		const { router } = await renderApp(
			'/points/lenina/deliveries/delivery-118'
		);

		await userEvent.click(
			await screen.findByRole('button', { name: 'История событий' })
		);

		const panel = await screen.findByRole('dialog', {
			name: 'История поставки № 118',
		});
		const events = await within(panel).findByRole('list', {
			name: 'События поставки',
		});

		expect(
			within(events)
				.getAllByRole('heading', { level: 3 })
				.map((heading) => heading.textContent)
		).toEqual([
			'Зарегистрирована',
			'Принята',
			'Приёмка исправлена',
			'Карантин решён: Маскарпоне',
		]);
		const [registered, approved] = within(events).getAllByRole('listitem');

		expect(within(registered).getByText(/· Генератор$/)).toBeInTheDocument();
		expect(within(approved).getByText(/· Анна Котова$/)).toBeInTheDocument();
		expect(
			within(events).getByText(
				/^принято 25,0 → 20,0\sкг, отклонено 0 → 5,0\sкг$/
			)
		).toBeInTheDocument();
		expect(
			within(events).getByText(/^со склада снято 5,0\sкг$/)
		).toBeInTheDocument();
		expect(
			within(events).getByText(
				/^принято 1,5\sкг — новая партия; отклонено 0,5\sкг$/
			)
		).toBeInTheDocument();

		act(() => router.history.back());

		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	});
});
