import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { apiClient } from '@shared/api';
import { withUnits } from '@shared/lib/test-utils';

import { lastParams } from './test-api';
import { hotCarbonara, ilyaActor, ordersApi, pinnedHot } from './test-orders';
import { cheesecake, omelet } from './test-kitchen';
import {
	manager,
	renderApp,
	rolePermissions,
	testProfile,
	testStaffPoint,
	waiter,
} from './test-router';

describe('горячие предложения', () => {
	it('объясняют причину по продукту: сколько под риском, срок и расход порциями', async () => {
		ordersApi({ hot: [hotCarbonara()] });
		await renderApp('/points/lenina/orders/hot', {
			profile: testProfile([manager]),
		});

		const row = await screen.findByRole('row', { name: /Паста карбонара/ });

		expect(within(row).getByText(withUnits('500 мл под риском'))).toBeVisible();
		expect(within(row).getByText('ещё 6 ч')).toBeVisible();
		expect(
			within(row).getByText(
				withUnits(
					'На порцию 80 мл: 5 порций израсходуют 400 мл из 500 мл · используется ещё в 1 блюде'
				)
			)
		).toBeVisible();
		expect(within(row).getByText('5 порций')).toBeVisible();
		expect(within(row).getByText('15 порций')).toBeVisible();
		expect(
			screen.getByRole('tab', { name: /^Горячие предложения\s*1$/ })
		).toBeVisible();
	});

	it('горизонт «7 дней» уходит в запрос и хранится в адресе', async () => {
		const get = ordersApi({ hot: [hotCarbonara()] });
		const { router } = await renderApp('/points/lenina/orders/hot');

		await screen.findByRole('row', { name: /Паста карбонара/ });
		expect(lastParams(get, '/recommendations/dishes')).toMatchObject({
			horizon_hours: 72,
		});
		expect(screen.getByRole('button', { name: '24 часа' })).toBeVisible();

		await userEvent.click(screen.getByRole('button', { name: '7 дней' }));

		await waitFor(() =>
			expect(lastParams(get, '/recommendations/dishes')).toMatchObject({
				horizon_hours: 168,
			})
		);
		expect(router.state.location.search).toMatchObject({ horizon: 168 });
	});

	it('без партий под риском — «Продвигать нечего» и предложение посмотреть на неделю', async () => {
		ordersApi({ hot: [] });
		await renderApp('/points/lenina/orders/hot');

		expect(await screen.findByText('Продвигать нечего')).toBeVisible();
		expect(
			screen.getByRole('button', { name: 'Показать на 7 дней' })
		).toBeVisible();
	});

	it('без права на рекомендации вкладки нет, а её адрес показывает «Нет доступа»', async () => {
		const noRecommendations = testStaffPoint('waiter', {
			permissions: rolePermissions.waiter.filter(
				(permission) => permission !== 'recommendations.view'
			),
		});

		ordersApi();
		await renderApp('/points/lenina/orders', {
			profile: testProfile([noRecommendations]),
		});

		await screen.findByRole('tab', { name: 'История' });
		expect(
			screen.queryByRole('tab', { name: /Горячие предложения/ })
		).toBeNull();

		await renderApp('/points/lenina/orders/hot', {
			profile: testProfile([noRecommendations]),
		});

		expect(
			await screen.findByRole('heading', { name: 'Нет доступа к разделу' })
		).toBeVisible();
	});
});

const put = vi.spyOn(apiClient, 'put');

function pinnedSoup() {
	const setAt = new Date();

	setAt.setHours(10, 42, 0, 0);

	return pinnedHot(
		{
			dish: { id: 'dish-soup', name: 'Тыквенный суп' },
			priority: 0,
			available_portions: 14,
			suggested_portions: 0,
			reasons: [],
		},
		'Блюдо дня: предлагайте к обеду',
		setAt
	);
}

const hotRows = () =>
	within(
		screen.getByRole('table', { name: 'Горячие предложения' })
	).getAllByRole('row');

describe('ручные горячие предложения', () => {
	beforeEach(() => {
		put.mockResolvedValue({ data: {} });
	});

	it('закреплённое блюдо первое с комментарием; без права управлять меню действий нет', async () => {
		ordersApi({ hot: [pinnedSoup(), hotCarbonara()] });
		await renderApp('/points/lenina/orders/hot', {
			profile: testProfile([waiter]),
		});

		const soup = await screen.findByRole('row', { name: /Тыквенный суп/ });

		expect(hotRows().indexOf(soup)).toBe(1);
		expect(soup).toHaveTextContent('закреплено');
		expect(soup).toHaveTextContent('«Блюдо дня: предлагайте к обеду»');
		expect(soup).toHaveTextContent('Илья Смирнов · 10:42');
		expect(screen.queryByRole('button', { name: 'Добавить блюдо' })).toBeNull();
		expect(screen.queryByRole('button', { name: /^Действия:/ })).toBeNull();
	});

	it('«Добавить блюдо» закрепляет блюдо меню с комментарием', async () => {
		ordersApi({ hot: [hotCarbonara()] });
		await renderApp('/points/lenina/orders/hot');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Добавить блюдо' })
		);

		const dialog = await screen.findByRole('dialog', {
			name: 'Добавить в горячие предложения',
		});

		await userEvent.click(
			within(dialog).getByRole('combobox', { name: 'Блюдо' })
		);
		await userEvent.click(await screen.findByRole('option', { name: 'Омлет' }));
		await userEvent.type(
			within(dialog).getByRole('textbox', {
				name: 'Комментарий для официанта',
			}),
			'Блюдо дня'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Закрепить' })
		);

		await waitFor(() =>
			expect(put).toHaveBeenCalledWith(
				`/points/lenina/recommendations/dishes/${encodeURIComponent(omelet.dish.id)}`,
				{ mode: 'pinned', comment: 'Блюдо дня' },
				expect.anything()
			)
		);
		expect(await screen.findByText('«Омлет» закреплено первым')).toBeVisible();
	});

	it('«Скрыть до конца дня» передаёт часовой пояс, «Вернуть» в сообщении возвращает расчёт', async () => {
		ordersApi({ hot: [hotCarbonara()] });
		await renderApp('/points/lenina/orders/hot');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Действия: Паста карбонара' })
		);
		await userEvent.click(
			await screen.findByRole('menuitem', { name: /Скрыть до конца дня/ })
		);

		await waitFor(() =>
			expect(put).toHaveBeenLastCalledWith(
				expect.stringMatching(/\/recommendations\/dishes\//),
				{ mode: 'hidden', time_zone: expect.any(String) },
				expect.anything()
			)
		);

		await userEvent.click(
			await screen.findByRole('button', { name: 'Вернуть' })
		);

		await waitFor(() =>
			expect(put).toHaveBeenLastCalledWith(
				expect.stringMatching(/\/recommendations\/dishes\//),
				{ mode: 'auto' },
				expect.anything()
			)
		);
	});

	it('скрытые сегодня блюда можно вернуть расчёту; без скрытых раздела нет', async () => {
		ordersApi({
			hot: [hotCarbonara()],
			settings: [
				{
					dish: cheesecake.dish,
					mode: 'hidden',
					comment: '',
					set_by: ilyaActor,
					set_at: new Date().toISOString(),
					hidden_until: new Date(
						new Date().setHours(23, 59, 0, 0)
					).toISOString(),
				},
			],
		});
		const { unmount } = await renderApp('/points/lenina/orders/hot');

		const hidden = await screen.findByRole('region', {
			name: 'Скрыто до конца дня',
		});

		expect(hidden).toHaveTextContent('до 23:59 · Илья Смирнов');

		await userEvent.click(
			within(hidden).getByRole('button', {
				name: `Вернуть в расчёт: ${cheesecake.dish.name}`,
			})
		);

		await waitFor(() =>
			expect(put).toHaveBeenCalledWith(
				`/points/lenina/recommendations/dishes/${encodeURIComponent(cheesecake.dish.id)}`,
				{ mode: 'auto' },
				expect.anything()
			)
		);
		unmount();

		ordersApi({ hot: [hotCarbonara()] });
		await renderApp('/points/lenina/orders/hot');
		await screen.findByRole('row', { name: /Паста карбонара/ });
		expect(
			screen.queryByRole('region', { name: 'Скрыто до конца дня' })
		).toBeNull();
	});
});
