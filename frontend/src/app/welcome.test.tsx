import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Profile } from '@entities/profile';
import { ApiError, apiClient } from '@shared/api';

import { idempotencyKeyOf, mockGet } from './test-api';
import { renderApp, testPoint, testProfile } from './test-router';

const noCompanies = testProfile([], []);
const companyWithoutPoints = testProfile([]);
const withPoint = testProfile([testPoint({ id: 'center', name: 'Центр' })]);

const post = vi.spyOn(apiClient, 'post');

/** Профиль `/me` на сервере: после записи тест задаёт новый. */
let serverProfile: Profile;

beforeEach(() => {
	serverProfile = noCompanies;
	mockGet((url) => {
		if (url === '/me') {
			return serverProfile;
		}

		throw new ApiError('Не найдено', { status: 404 });
	});
});

describe('первый вход', () => {
	it('пустое название не отправляется', async () => {
		await renderApp('/welcome', { profile: noCompanies });

		await userEvent.click(
			await screen.findByRole('button', { name: 'Продолжить' })
		);

		expect(screen.getByText('Введите название компании')).toBeInTheDocument();
		expect(post).not.toHaveBeenCalled();
	});

	it('после компании и точки открывает «Сводку» новой точки', async () => {
		const { router } = await renderApp('/welcome', { profile: noCompanies });

		post.mockResolvedValueOnce({ data: { id: 'demo' } });
		serverProfile = companyWithoutPoints;
		await userEvent.type(
			await screen.findByRole('textbox', { name: /Название компании/ }),
			'Кафе у моста'
		);
		await userEvent.click(screen.getByRole('button', { name: 'Продолжить' }));

		post.mockResolvedValueOnce({ data: { id: 'center' } });
		serverProfile = withPoint;
		await userEvent.type(
			await screen.findByRole('textbox', { name: /Название точки/ }),
			'Центр'
		);
		await userEvent.click(
			screen.getByRole('button', { name: 'Создать точку' })
		);

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/center')
		);
		expect(post).toHaveBeenLastCalledWith(
			'/companies/demo/points',
			{ name: 'Центр' },
			expect.anything()
		);
	});

	it('владельцу компании без точек сразу предлагает создать точку', async () => {
		await renderApp('/welcome', { profile: companyWithoutPoints });

		expect(
			await screen.findByRole('heading', { name: 'Добавьте первую точку' })
		).toBeInTheDocument();
	});

	it('повтор после сбоя отправляет тот же ключ идемпотентности', async () => {
		await renderApp('/welcome', { profile: noCompanies });

		post.mockRejectedValueOnce(
			new ApiError('Нет связи с сервером. Проверьте подключение и повторите')
		);
		await userEvent.type(
			await screen.findByRole('textbox', { name: /Название компании/ }),
			'Кафе у моста'
		);
		await userEvent.click(screen.getByRole('button', { name: 'Продолжить' }));
		expect(
			await screen.findByText(
				'Нет связи с сервером. Проверьте подключение и повторите'
			)
		).toBeInTheDocument();

		post.mockResolvedValueOnce({ data: { id: 'demo' } });
		serverProfile = companyWithoutPoints;
		await userEvent.click(screen.getByRole('button', { name: 'Продолжить' }));

		await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
		const keys = post.mock.calls.map(idempotencyKeyOf);

		expect(keys[0]).toBe(keys[1]);
	});

	it('«Проверить снова» уводит на точку, когда сотрудника добавили', async () => {
		const { router } = await renderApp('/welcome', { profile: noCompanies });

		serverProfile = withPoint;
		await userEvent.click(
			await screen.findByRole('button', { name: 'Проверить снова' })
		);

		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/center')
		);
	});
});
