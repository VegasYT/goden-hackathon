import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { apiClient } from '@shared/api';
import { withUnits } from '@shared/lib/test-utils';

import { lastListParams, requestsTo } from './test-api';
import { approver, renderApp, testProfile } from './test-router';
import {
	fillWriteOff,
	inHours,
	stockApi,
	testBatch,
	testExpiring,
	testProduct,
	testStockSummary,
	tomato,
} from './test-stock';

const post = vi.spyOn(apiClient, 'post');
const salmon = testProduct({
	id: 'salmon',
	name: 'Лосось охлаждённый',
	category: 'Рыба и морепродукты',
});

const expiredSalmon = testExpiring(salmon, {
	id: 'b-109',
	delivery_id: 'd-109',
	delivery_number: '109',
	quantity: 400,
	expires_at: inHours(-0.1),
});
const tomatoSoon = testExpiring(tomato, { expires_at: inHours(17.5) });

describe('вкладка «Сроки»', () => {
	it('показывает партии от ближайшего срока: просроченную нужно списать, срок меньше суток выделен', async () => {
		const get = stockApi({
			expiring: () => ({ items: [expiredSalmon, tomatoSoon], total: 2 }),
		});

		await renderApp('/points/lenina/stock/expiring');

		const table = await screen.findByRole('table', {
			name: 'Истекающие партии',
		});
		const rows = within(table).getAllByRole('row');
		const salmonRow = within(table).getByRole('row', {
			name: /Лосось охлаждённый/,
		});
		const tomatoRow = within(table).getByRole('row', { name: /Томаты черри/ });

		expect(rows.indexOf(salmonRow)).toBeLessThan(rows.indexOf(tomatoRow));
		expect(salmonRow).toHaveTextContent('просрочена');
		expect(salmonRow).toHaveTextContent('нужно списать');
		expect(tomatoRow).toHaveTextContent('ещё 17 ч');
		expect(
			within(tomatoRow).getByRole('link', { name: 'Поставка № 111' })
		).toHaveAttribute('href', '/points/lenina/deliveries/d-111');
		expect(lastListParams(get, '/batches/expiring')).toEqual({
			within_hours: 72,
			limit: 10,
			offset: 0,
		});
	});

	it('просроченная партия списывается целиком с причиной «Истёк срок годности» без комментария', async () => {
		stockApi({
			expiring: () => ({ items: [expiredSalmon, tomatoSoon], total: 2 }),
			batches: () => [expiredSalmon.batch],
		});
		post.mockResolvedValueOnce({ data: {} });
		await renderApp('/points/lenina/stock/expiring');

		await userEvent.click(
			await screen.findByRole('button', {
				name: 'Списать партию № 109: Лосось охлаждённый',
			})
		);

		const dialog = await screen.findByRole('dialog', { name: 'Списать порчу' });

		expect(
			within(dialog).getByRole('radio', { name: 'Истёк срок годности' })
		).toBeChecked();

		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 400 г') })
		);

		await waitFor(() =>
			expect(post).toHaveBeenCalledWith(
				'/points/lenina/batches/b-109/write-offs',
				{ quantity: 400, reason: 'expired', comment: '' },
				expect.anything()
			)
		);
	});

	it('плашка над списком называет число продуктов с просрочкой, без просрочки её нет', async () => {
		stockApi({
			summary: testStockSummary({ expired: 2 }),
			expiring: () => ({ items: [expiredSalmon], total: 1 }),
		});
		const { unmount } = await renderApp('/points/lenina/stock/expiring');

		expect(
			await screen.findByText(/Просрочка у 2 продуктов/)
		).toBeInTheDocument();
		unmount();

		stockApi({ expiring: () => ({ items: [tomatoSoon], total: 1 }) });
		await renderApp('/points/lenina/stock/expiring');

		await screen.findByRole('table', { name: 'Истекающие партии' });
		expect(screen.queryByText(/Просрочка у/)).toBeNull();
	});

	it('смена периода уходит на сервер и в адрес, страница сбрасывается', async () => {
		const get = stockApi({
			expiring: () => ({ items: [tomatoSoon], total: 1 }),
		});
		const { router } = await renderApp('/points/lenina/stock/expiring?page=2');

		await userEvent.click(
			await screen.findByRole('button', { name: '7 дней' })
		);

		await waitFor(() =>
			expect(lastListParams(get, '/batches/expiring')).toEqual({
				within_hours: 168,
				limit: 10,
				offset: 0,
			})
		);
		expect(router.state.location.search).toEqual({ within: 168 });
	});

	it('за период ничего не истекает — предлагает 7 дней', async () => {
		stockApi();
		const { router } = await renderApp('/points/lenina/stock/expiring');

		expect(
			await screen.findByText('За 3 дня ничего не истекает')
		).toBeInTheDocument();

		await userEvent.click(
			screen.getByRole('button', { name: 'Показать за 7 дней' })
		);

		await waitFor(() =>
			expect(router.state.location.search).toEqual({ within: 168 })
		);
		expect(
			await screen.findByText('За 7 дней ничего не истекает')
		).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /^Показать за/ })).toBeNull();
	});

	it('название продукта открывает историю поверх «Сроков»', async () => {
		stockApi({ expiring: () => ({ items: [tomatoSoon], total: 1 }) });
		const { router } = await renderApp('/points/lenina/stock/expiring');

		await userEvent.click(
			await screen.findByRole('link', { name: 'Томаты черри' })
		);

		expect(
			await screen.findByRole('dialog', { name: 'История · Томаты черри' })
		).toBeInTheDocument();
		expect(router.state.location.pathname).toBe(
			'/points/lenina/stock/expiring'
		);
		expect(router.state.location.search).toEqual({ history: 'tomato' });

		await userEvent.click(
			screen.getByRole('button', { name: 'Закрыть историю' })
		);

		await waitFor(() => expect(router.state.location.search).toEqual({}));
		expect(router.state.location.pathname).toBe(
			'/points/lenina/stock/expiring'
		);
	});

	it('после списания перечитывает список и число на вкладке', async () => {
		post.mockResolvedValueOnce({ data: testBatch({ quantity: 1100 }) });
		const get = stockApi({
			expiring: () => ({ items: [testExpiring(tomato, {})], total: 1 }),
		});

		await renderApp('/points/lenina/stock/expiring');
		await userEvent.click(
			await screen.findByRole('button', {
				name: 'Списать партию № 111: Томаты черри',
			})
		);

		const dialog = await screen.findByRole('dialog', { name: 'Списать порчу' });

		await fillWriteOff(dialog, 'Помялись');
		const before = requestsTo(get, '/batches/expiring').length;

		await userEvent.click(
			within(dialog).getByRole('button', { name: withUnits('Списать 500 г') })
		);

		expect(
			await screen.findByText(
				'Списано 500 г · Томаты черри, партия № 111. Осталось 1,1 кг'
			)
		).toBeVisible();
		// Перечитаны и список вкладки, и число партий за сутки.
		await waitFor(() =>
			expect(
				requestsTo(get, '/batches/expiring').length
			).toBeGreaterThanOrEqual(before + 2)
		);
	});

	it('у приёмщика нет «Списать», есть пояснение', async () => {
		stockApi({ expiring: () => ({ items: [tomatoSoon], total: 1 }) });
		await renderApp('/points/lenina/stock/expiring', {
			profile: testProfile([approver]),
		});

		expect(
			await screen.findByRole('link', { name: 'Томаты черри' })
		).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /^Списать/ })).toBeNull();
		expect(screen.getByRole('note')).toHaveTextContent(
			'Списывать и исправлять остаток могут управляющий и владелец'
		);
	});
});
