import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Delivery } from '@entities/delivery';
import type { DishAvailability, HotDish } from '@entities/dish';
import type { ExpiringBatch } from '@entities/stock';
import { ApiError } from '@shared/api';
import { withUnits } from '@shared/lib/test-utils';

import { mockGet, requestsTo } from './test-api';
import { testDelivery, testSummary } from './test-deliveries';
import {
	bruschetta,
	carbonara,
	omelet,
	testDish,
	testIngredient,
	testMenu,
} from './test-kitchen';
import { hotCarbonara, pinnedHot } from './test-orders';
import { connect, signal } from './test-realtime';
import { cook, renderApp, testProfile } from './test-router';
import { inHours, testExpiring, testProduct } from './test-stock';

const now = new Date(2026, 9, 14, 14, 39);

const batchOf = (name: string, hours: number, index: number) =>
	testExpiring(testProduct({ id: `product-${index}`, name }), {
		id: `batch-${index}`,
		delivery_number: String(110 + index),
		quantity: 1200,
		expires_at: inHours(hours),
	});

/** Шесть ближайших партий: первая уже просрочена. Строятся после установки времени теста. */
const nearest = () => [
	batchOf('Лосось охлаждённый', -1, 1),
	batchOf('Сливки 33%', 6.5, 2),
	batchOf('Творог 9%', 9, 3),
	batchOf('Томаты черри', 17, 4),
	batchOf('Куриное филе', 19, 5),
	batchOf('Базилик свежий', 22, 6),
];

interface DashboardApi {
	menu: DishAvailability[] | 'error';
	expiring: ExpiringBatch[];
	expiringTotal: number;
	hot: HotDish[];
	hotTotal: number;
	pending: Delivery[];
	openQuarantineItems: number;
	quarantineDeliveries: number;
}

function dashboardApi(overrides: Partial<DashboardApi> = {}) {
	const state: DashboardApi = {
		menu: testMenu,
		expiring: nearest(),
		expiringTotal: 13,
		hot: [hotCarbonara(now)],
		hotTotal: 4,
		pending: [testDelivery({ delivered_at: '2026-10-14T06:10:00Z' })],
		openQuarantineItems: 3,
		quarantineDeliveries: 2,
		...overrides,
	};

	const get = mockGet((url, params) => {
		if (url.endsWith('/menu')) {
			if (state.menu === 'error') {
				throw new ApiError('Сервер недоступен', { status: 500 });
			}

			return { items: state.menu, total: state.menu.length };
		}

		if (url.endsWith('/batches/expiring')) {
			return {
				items: state.expiring,
				total: state.expiringTotal,
			};
		}

		if (url.endsWith('/recommendations/dishes')) {
			return { items: state.hot, total: state.hotTotal };
		}

		if (url.endsWith('/deliveries/summary')) {
			return testSummary({ open_quarantine_items: state.openQuarantineItems });
		}

		if (url.endsWith('/deliveries')) {
			if (params.quarantine === 'open') {
				return { items: [], total: state.quarantineDeliveries };
			}

			return params.status === 'registered'
				? { items: state.pending.slice(0, 1), total: state.pending.length }
				: { items: [], total: 0 };
		}

		throw new ApiError('Не найдено', { status: 404 });
	});

	return { get, state };
}

/** Счётчик ищется в содержимом страницы: в навигации есть пункт с той же подписью. */
const tile = (label: string) =>
	within(screen.getByRole('main')).findByRole('link', {
		name: new RegExp(`^${label}`),
	});

/** Проверяет число, слово и подпись счётчика по отдельности. */
async function expectTile(label: string, ...parts: string[]) {
	const found = await tile(label);

	for (const part of parts) {
		expect(within(found).getByText(part)).toBeInTheDocument();
	}

	return found;
}

const panel = (title: string) => screen.getByRole('region', { name: title });

beforeEach(() => {
	vi.useFakeTimers({ shouldAdvanceTime: true });
	vi.setSystemTime(now);
});

describe('сводка', () => {
	it('управляющему показывает пять счётчиков с подписями и три списка', async () => {
		dashboardApi();
		await renderApp('/points/lenina');

		await expectTile(
			'Истекают за сутки',
			'13',
			'партий',
			'1 просрочена, нужно списать'
		);
		await expectTile('Стоп-лист', '3', 'блюда', 'из 5 блюд меню');
		await expectTile('Горячие предложения', '4', 'блюда');
		await expectTile('Ожидают приёмки', '1', 'поставка');
		await expectTile('Открытый карантин', '3', 'позиции', 'в 2 поставках');

		const stopList = panel('Стоп-лист');

		expect(
			await within(stopList).findByText(
				withUnits('Не хватает: Томаты · есть 60 г, на порцию 100 г')
			)
		).toBeVisible();
		expect(within(stopList).getByText('Брускетта с томатами')).toBeVisible();

		const hot = panel('Предложить гостям');

		expect(within(hot).getByText('Паста карбонара')).toBeVisible();
		expect(within(hot).getByText('Сливки 33% · ещё 6 ч')).toBeVisible();
		expect(within(hot).getByText('5 порций')).toBeVisible();

		const soon = panel('Ближайшие сроки');

		expect(within(soon).getAllByRole('row')).toHaveLength(7);
		expect(
			within(soon).getByRole('link', { name: 'и ещё 7 партий за сутки' })
		).toBeVisible();
	});

	it('закреплённое блюдо в «Предложить гостям» с отметкой и комментарием', async () => {
		dashboardApi({
			hot: [pinnedHot(hotCarbonara(now), 'Блюдо дня', now)],
		});
		await renderApp('/points/lenina');

		const hot = panel('Предложить гостям');

		expect(await within(hot).findByText('закреплено')).toBeVisible();
		expect(within(hot).getByText('«Блюдо дня»')).toBeVisible();
		expect(within(hot).queryByText('5 порций')).toBeNull();
	});

	it('счётчик «Открытый карантин» открывает панель карантина в «Поставках»', async () => {
		dashboardApi();
		const { router } = await renderApp('/points/lenina');

		await userEvent.click(await tile('Открытый карантин'));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/lenina/deliveries')
		);
		expect(router.state.location.search).toMatchObject({ panel: 'quarantine' });
	});

	it('повару показывает только стоп-лист и не запрашивает чужие данные', async () => {
		const { get } = dashboardApi();

		await renderApp('/points/lenina', { profile: testProfile([cook]) });

		await tile('Стоп-лист');
		expect(panel('Стоп-лист')).toBeVisible();
		for (const hidden of ['Ближайшие сроки', 'Предложить гостям']) {
			expect(
				screen.queryByRole('region', { name: hidden })
			).not.toBeInTheDocument();
		}

		expect(
			within(screen.getByRole('main')).queryAllByRole('link', {
				name: /^(Истекают|Горячие|Ожидают|Открытый)/,
			})
		).toHaveLength(0);

		for (const suffix of [
			'/batches/expiring',
			'/recommendations/dishes',
			'/deliveries',
			'/deliveries/summary',
		]) {
			expect(requestsTo(get, suffix)).toHaveLength(0);
		}
	});

	it('при пустых ответах нули показаны как порядок', async () => {
		dashboardApi({
			menu: [carbonara, omelet],
			expiring: [],
			expiringTotal: 0,
			hot: [],
			hotTotal: 0,
			pending: [],
			openQuarantineItems: 0,
			quarantineDeliveries: 0,
		});
		await renderApp('/points/lenina');

		expect(await tile('Истекают за сутки')).toHaveTextContent(
			'за сутки ничего не истекает'
		);
		await expectTile('Стоп-лист', '0', 'блюд', 'всё меню можно приготовить');
		await expectTile(
			'Горячие предложения',
			'0',
			'блюд',
			'за 3 дня ничего не истекает'
		);
		expect(await tile('Ожидают приёмки')).toHaveTextContent(
			'поставок на приёмке нет'
		);
		expect(await tile('Открытый карантин')).toHaveTextContent(
			'решений не ждёт'
		);
		expect(
			within(panel('Стоп-лист')).getByText('Все блюда меню можно приготовить')
		).toBeVisible();
		expect(
			within(panel('Ближайшие сроки')).getByText('За сутки ничего не истекает')
		).toBeVisible();
	});

	it('ошибка меню не мешает остальным блокам, «Повторить» загружает стоп-лист', async () => {
		const { state } = dashboardApi({ menu: 'error' });

		await renderApp('/points/lenina');

		const stopList = panel('Стоп-лист');

		expect(
			await within(stopList).findByText('Не удалось загрузить стоп-лист')
		).toBeVisible();
		expect(
			within(screen.getByRole('group', { name: 'Стоп-лист' })).getByText(
				'Не загрузилось'
			)
		).toBeVisible();
		await expectTile('Истекают за сутки', '13', 'партий');

		state.menu = testMenu;
		await userEvent.click(
			within(stopList).getByRole('button', { name: 'Повторить' })
		);

		expect(
			await within(stopList).findByText('Брускетта с томатами')
		).toBeVisible();
		await expectTile('Стоп-лист', '3', 'блюда');
	});

	it('сигнал о стоп-листе перечитывает меню, новое блюдо видно в счётчике и списке', async () => {
		const { state } = dashboardApi({ menu: [bruschetta, omelet] });

		await renderApp('/points/lenina');

		await expectTile('Стоп-лист', '1', 'блюдо');

		connect();
		state.menu = [
			bruschetta,
			testDish('Омлет', [testIngredient('Яйцо куриное', 'pcs', 3, 2)]),
		];
		signal(['stop-list']);

		const stopTile = await tile('Стоп-лист');

		await waitFor(() =>
			expect(within(stopTile).getByText('2')).toBeInTheDocument()
		);
		expect(within(stopTile).getByText('блюда')).toBeInTheDocument();
		expect(within(panel('Стоп-лист')).getByText('Омлет')).toBeVisible();
	});
});
