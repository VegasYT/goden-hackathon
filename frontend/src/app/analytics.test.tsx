import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { ApiError, type components } from '@shared/api';
import { rowCells, typeDate } from '@shared/lib/test-utils';

import { mockGet } from './test-api';
import { renderApp } from './test-router';
import { testProduct } from './test-stock';

const product = (name: string, unit: 'g' | 'pcs' = 'g') =>
	testProduct({ id: name, name, unit });

const tomatoes = product('Томаты');
const chicken = product('Куриное филе');
const eggs = product('Яйцо куриное', 'pcs');

const minutesFromNow = (minutes: number) =>
	new Date(Date.now() + minutes * 60_000).toISOString();

const forecast: components['schemas']['StopListForecast'][] = [
	{
		dish: { id: 'bruschetta', name: 'Брускетта с томатами' },
		limiting_product: tomatoes,
		expected_at: minutesFromNow(250.5),
		available_quantity: 1200,
		required_quantity: 80,
		consumption_per_hour: 290,
		cause: 'consumption',
	},
	{
		dish: { id: 'fillet', name: 'Куриное филе в сливочном соусе' },
		limiting_product: chicken,
		expected_at: minutesFromNow(31 * 60 + 0.5),
		available_quantity: 4600,
		required_quantity: 150,
		consumption_per_hour: 0,
		cause: 'expiry',
	},
];

const shortages: components['schemas']['ShortageStatistic'][] = [
	{
		product: eggs,
		rejected_orders: 14195,
		stop_entries: 1365,
		stop_seconds: 82860,
	},
];

const writeOffs: components['schemas']['WriteOffStatistic'][] = [
	{ product: tomatoes, reason: 'expired', quantity: 2100, operations: 2 },
	{
		product: tomatoes,
		reason: 'transport_damage',
		quantity: 1800,
		operations: 1,
	},
];

const today = new Date();
const day = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;

const writeOffDays: components['schemas']['WriteOffDay'][] = writeOffs.map(
	(item) => ({ ...item, day })
);

interface Requests {
	forecast: Record<string, unknown>[];
	period: Record<string, unknown>[];
}

/** API аналитики; `failShortages` - дефицит отвечает ошибкой. */
function analyticsApi({ failShortages = false, emptyForecast = false } = {}) {
	const requests: Requests = { forecast: [], period: [] };

	mockGet((url, params) => {
		if (url.endsWith('/stop-list-forecast')) {
			requests.forecast.push(params);
			return { items: emptyForecast ? [] : forecast };
		}

		if (url.endsWith('/shortages')) {
			requests.period.push(params);
			if (failShortages) {
				throw new ApiError('Сервер недоступен', { status: 503 });
			}

			return { items: shortages };
		}

		if (url.endsWith('/write-offs')) {
			return { items: writeOffs };
		}

		if (url.endsWith('/write-offs/days')) {
			return { items: writeOffDays };
		}

		throw new Error(`Неожиданный запрос ${url}`);
	});

	return requests;
}

const section = (name: string) => screen.getByRole('region', { name });

describe('аналитика', () => {
	it('прогноз объясняет причину, сбой дефицита не скрывает остальные блоки', async () => {
		analyticsApi({ failShortages: true });
		await renderApp('/points/lenina/analytics');

		const bruschetta = await screen.findByRole('listitem', {
			name: 'Брускетта с томатами',
		});

		expect(bruschetta).toHaveTextContent('хватит примерно на 15 порций');
		expect(bruschetta).toHaveTextContent(
			'Томаты: осталось 1,2 кг · около 290 г в час · 80 г на порцию'
		);
		expect(bruschetta).toHaveTextContent('через 4 ч 10 мин');
		expect(
			screen.getByRole('listitem', { name: 'Куриное филе в сливочном соусе' })
		).toHaveTextContent('осталось 4,6 кг · истечёт срок партии');
		expect(
			within(bruschetta).getByRole('link', { name: 'Томаты' })
		).toHaveAttribute(
			'href',
			expect.stringContaining('/points/lenina/stock?history=')
		);

		expect(
			await within(section('Дефицитные ингредиенты')).findByText(
				'Не удалось загрузить дефицитные ингредиенты'
			)
		).toBeInTheDocument();

		const tomatoes = within(section('Списания')).getByRole('row', {
			name: /^Томаты/,
		});

		expect(rowCells(tomatoes)).toEqual([
			'Томаты',
			expect.any(String),
			'3,9 кг',
			'3',
		]);
		expect(within(tomatoes).getByText('Истёк срок годности')).toBeVisible();
		expect(
			within(tomatoes).getByText('Повреждено при перевозке')
		).toBeVisible();
	});

	it('горизонт прогноза перечитывает прогноз и сохраняется в адресе', async () => {
		const requests = analyticsApi();
		const { router } = await renderApp('/points/lenina/analytics');

		await userEvent.click(
			await screen.findByRole('button', { name: '24 часа' })
		);

		expect(router.state.location.search).toMatchObject({ horizon: 24 });
		expect(requests.forecast.at(-1)).toEqual({ horizon_hours: 24 });
	});

	it('пустой прогноз называет выбранный горизонт', async () => {
		analyticsApi({ emptyForecast: true });
		await renderApp('/points/lenina/analytics');

		expect(
			await within(section('Прогноз стоп-листа')).findByRole('heading', {
				name: 'За 7 дней блюда не закончатся',
			})
		).toBeVisible();
	});

	it('свой период: начало позже конца не применяется, верный период попадает в адрес', async () => {
		analyticsApi();
		const { router } = await renderApp('/points/lenina/analytics');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Свой период' })
		);
		const dialog = screen.getByRole('dialog', { name: 'Свой период' });

		await typeDate('С', '20.10.2026');
		await typeDate('По', '14.10.2026');
		expect(
			within(dialog).getByText('Конец периода раньше начала')
		).toBeInTheDocument();
		expect(
			within(dialog).getByRole('button', { name: 'Показать' })
		).toBeDisabled();

		await typeDate('С', '01.10.2026');
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Показать' })
		);

		expect(router.state.location.search).toMatchObject({
			period: 'custom',
			from: '2026-10-01',
			to: '2026-10-14',
		});
		expect(await screen.findByText('1–14 октября')).toBeInTheDocument();
	});

	it('вид списаний по причинам из адреса', async () => {
		analyticsApi();
		await renderApp('/points/lenina/analytics?writeOffs=reasons');

		const row = await within(section('Списания')).findByRole('row', {
			name: /^Повреждено при перевозке/,
		});

		expect(rowCells(row)).toEqual([
			'Повреждено при перевозке',
			'1',
			'Томаты 1,8 кг',
		]);
	});

	it('продукт графика по дням сохраняется в адресе', async () => {
		analyticsApi();
		const { router } = await renderApp('/points/lenina/analytics');

		await userEvent.click(
			await within(section('Списания по дням')).findByRole('combobox', {
				name: 'Продукт',
			})
		);
		await userEvent.click(screen.getByRole('option', { name: 'Томаты' }));

		expect(router.state.location.search).toMatchObject({
			dayProduct: 'Томаты',
		});
		expect(
			within(section('Списания по дням')).getByText(
				'Сколько списано за день: Томаты'
			)
		).toBeInTheDocument();
	});
});
