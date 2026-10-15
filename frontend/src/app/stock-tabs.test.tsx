import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { requestsTo } from './test-api';
import { approver, renderApp, testProfile } from './test-router';
import { stockApi } from './test-stock';

const approverProfile = testProfile([approver]);

/** Число истекающих за сутки партий и рекомендаций, как их отдаёт сервер. */
const countedApi = () =>
	stockApi({
		expiring: (params) => ({
			items: [],
			total: params.within_hours === 24 ? 4 : 7,
		}),
		recommendations: () => ({ items: [], total: 5 }),
	});

describe('вкладки склада', () => {
	it('показывают числа и переключаются по адресу', async () => {
		countedApi();
		const { router } = await renderApp('/points/lenina/stock');

		expect(
			await screen.findByRole('tab', { name: 'Сроки 4' })
		).toBeInTheDocument();
		expect(
			await screen.findByRole('tab', { name: 'Пора заказать 5' })
		).toBeInTheDocument();
		expect(screen.getByRole('tab', { name: 'Остатки' })).toHaveAttribute(
			'aria-selected',
			'true'
		);

		await userEvent.click(screen.getByRole('tab', { name: 'Сроки 4' }));

		await waitFor(() =>
			expect(router.state.location.pathname).toBe(
				'/points/lenina/stock/expiring'
			)
		);
		expect(screen.getByRole('tab', { name: 'Сроки 4' })).toHaveAttribute(
			'aria-selected',
			'true'
		);
	});

	it('приёмщик не видит «Пора заказать», рекомендации не запрашиваются', async () => {
		const get = countedApi();
		const view = await renderApp('/points/lenina/stock', {
			profile: approverProfile,
		});

		expect(
			await screen.findByRole('tab', { name: 'Сроки 4' })
		).toBeInTheDocument();
		expect(screen.queryByRole('tab', { name: /Пора заказать/ })).toBeNull();
		expect(requestsTo(get, /\/recommendations/)).toEqual([]);
		view.unmount();

		await renderApp('/points/lenina/stock/reorder', {
			profile: approverProfile,
		});

		expect(
			await screen.findByText('Нет доступа к разделу')
		).toBeInTheDocument();
	});
});
