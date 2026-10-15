import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, apiClient } from '@shared/api';

import { idempotencyKeyOf, mockGet, requestsTo } from './test-api';
import { renderApp, testPoint, testProfile, testSession } from './test-router';

const post = vi.spyOn(apiClient, 'post');

const embankment = {
	id: 'embankment',
	name: 'Кафе на набережной',
	owner_id: testSession.userId,
	active: true,
};
const demo = {
	id: 'demo',
	name: 'Демо',
	owner_id: testSession.userId,
	active: true,
};

const withNewCompany = testProfile(
	[
		testPoint(),
		testPoint({
			id: 'center',
			company_id: 'embankment',
			company_name: 'Кафе на набережной',
			name: 'Центр',
		}),
	],
	[demo, embankment]
);

/** Профиль на сервере: после компании в нём компания без точек, после точки — с точкой. */
const withEmptyCompany = testProfile([testPoint()], [demo, embankment]);

beforeEach(() => {
	mockGet(() =>
		post.mock.calls.length >= 2 ? withNewCompany : withEmptyCompany
	);
});

async function openDialog() {
	await userEvent.click(
		await screen.findByRole('button', { name: /Сменить точку/ })
	);
	await userEvent.click(
		screen.getByRole('menuitem', { name: 'Новая компания' })
	);

	return screen.getByRole('dialog');
}

describe('новая компания', () => {
	it('создаёт компанию с первой точкой и открывает её «Сводку»', async () => {
		const { router } = await renderApp('/points/lenina');
		const dialog = await openDialog();

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название компании' }),
			'Кафе на набережной'
		);
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Первая точка' }),
			'Центр'
		);
		post
			.mockResolvedValueOnce({ data: embankment })
			.mockResolvedValueOnce({ data: { id: 'center' } });
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать компанию' })
		);

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/center')
		);
		expect(post).toHaveBeenNthCalledWith(
			1,
			'/companies',
			{ name: 'Кафе на набережной' },
			expect.anything()
		);
		expect(post).toHaveBeenNthCalledWith(
			2,
			'/companies/embankment/points',
			{ name: 'Центр' },
			expect.anything()
		);
	});

	it('после сбоя точки повтор не создаёт вторую компанию', async () => {
		await renderApp('/points/lenina');
		const dialog = await openDialog();

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название компании' }),
			'Кафе на набережной'
		);
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Первая точка' }),
			'Центр'
		);
		post
			.mockResolvedValueOnce({ data: embankment })
			.mockRejectedValueOnce(
				new ApiError('Нет связи с сервером. Проверьте подключение и повторите')
			);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать компанию' })
		);

		expect(
			await within(dialog).findByText(/Нет связи с сервером/)
		).toBeInTheDocument();
		post.mockResolvedValueOnce({ data: { id: 'center' } });
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать точку' })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(3));
		expect(requestsTo(post, /^\/companies$/)).toHaveLength(1);
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[2])
		);
	});

	it('для своей компании без точек предлагает только точку', async () => {
		await renderApp('/points/lenina', {
			profile: testProfile([testPoint()], [demo, embankment]),
		});
		const dialog = await openDialog();

		expect(
			within(dialog).getByRole('heading', {
				name: 'Первая точка компании «Кафе на набережной»',
			})
		).toBeInTheDocument();
		expect(
			within(dialog).queryByRole('textbox', { name: 'Название компании' })
		).toBeNull();
	});
});
