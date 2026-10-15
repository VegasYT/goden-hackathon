import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { rowCells } from '@shared/lib/test-utils';

import { renderApp } from './test-router';
import { stockApi, testProduct } from './test-stock';

const dish = (name: string) => ({ id: `dish-${name}`, name });

const arugula = {
	product: testProduct({ id: 'arugula', name: 'Руккола', unit: 'g' }),
	available_quantity: 150,
	consumed_quantity: 400,
	cover_hours: 9.4,
	suggested_quantity: 1050,
	reason: 'low_cover',
	needed_by: [dish('Салат с рукколой')],
};

const parmesan = {
	product: testProduct({ id: 'parmesan', name: 'Пармезан', unit: 'g' }),
	available_quantity: 20,
	consumed_quantity: 0,
	suggested_quantity: 300,
	reason: 'out_of_stock',
	needed_by: [dish('Паста карбонара')],
};

const eggs = {
	product: testProduct({ id: 'eggs', name: 'Яйца куриные С0', unit: 'pcs' }),
	available_quantity: 24,
	consumed_quantity: 60,
	cover_hours: 10,
	suggested_quantity: 156,
	reason: 'low_cover',
	needed_by: [dish('Омлет'), dish('Сырники'), dish('Паста карбонара')],
};

const row = (name: string) =>
	screen.getByRole('row', { name: new RegExp(`^${name}`) });

describe('вкладка «Пора заказать»', () => {
	it('показывает, на сколько хватит, сколько заказать и для каких блюд', async () => {
		stockApi({
			recommendations: () => ({
				items: [parmesan, arugula, eggs],
				total: 3,
			}),
		});
		await renderApp('/points/lenina/stock/reorder');

		await screen.findByRole('table', { name: 'Рекомендации поставок' });

		expect(rowCells(row('Руккола'))).toEqual([
			'Руккола',
			'150 г',
			'400 г',
			'9 ч',
			'1,05 кг',
			'Салат с рукколой',
		]);
		expect(rowCells(row('Пармезан'))).toEqual([
			'Пармезан',
			'20 г',
			'не было',
			'—',
			'300 г',
			expect.any(String),
		]);
		expect(row('Яйца куриные С0')).toHaveTextContent('Омлет, Сырники и ещё 1');
		expect(
			within(row('Яйца куриные С0')).getByLabelText('и ещё 1: Паста карбонара')
		).toBeInTheDocument();
	});

	it('пояснение расчёта свёрнуто и раскрывается по «Как считаем»', async () => {
		stockApi({ recommendations: () => ({ items: [arugula], total: 1 }) });
		await renderApp('/points/lenina/stock/reorder');

		const toggle = await screen.findByRole('button', { name: 'Как считаем' });

		expect(toggle).toHaveAttribute('aria-expanded', 'false');
		expect(
			screen.queryByText(/предлагаем запас на 10 порций/)
		).not.toBeVisible();

		await userEvent.click(toggle);

		expect(toggle).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByText(/предлагаем запас на 10 порций/)).toBeVisible();
	});

	it('без рекомендаций говорит, что заказывать нечего', async () => {
		stockApi();
		await renderApp('/points/lenina/stock/reorder');

		expect(
			await screen.findByText('Заказывать пока нечего')
		).toBeInTheDocument();
	});
});
