import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { MigomOnPoint, Point } from '@entities/organization';
import type { Profile } from '@entities/profile';
import { apiClient } from '@shared/api';

import { mockGet } from './test-api';
import {
	manager,
	renderApp,
	testOrgPoint,
	testPoint,
	testProfile,
} from './test-router';

const post = vi.spyOn(apiClient, 'post');
const put = vi.spyOn(apiClient, 'put');
const patch = vi.spyOn(apiClient, 'patch');

const lenina = testOrgPoint();

/** Профиль, точка и подключение Мигома на сервере: после записи тест задаёт новые. */
let server: { profile: Profile; point: Point; migom: MigomOnPoint };

const managerProfile = testProfile([manager]);

beforeEach(() => {
	server = {
		profile: testProfile(),
		point: lenina,
		migom: { outlet_id: null },
	};
	mockGet((url) => {
		if (url === '/me') {
			return server.profile;
		}

		return url.endsWith('/migom') ? server.migom : server.point;
	});
});

describe('настройки точки', () => {
	it('владелец переименовывает точку, новое название видно в заголовке', async () => {
		await renderApp('/points/lenina/settings');

		const field = await screen.findByRole('textbox', { name: 'Название' });

		await userEvent.clear(field);
		await userEvent.type(field, 'Центр');
		patch.mockResolvedValueOnce({ data: { ...lenina, name: 'Центр' } });
		server.profile = testProfile([testPoint({ name: 'Центр' })]);
		await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

		expect(patch).toHaveBeenCalledWith(
			'/points/lenina',
			{ name: 'Центр' },
			expect.anything()
		);
		expect(
			await screen.findByText('Центр · компания «Демо»')
		).toBeInTheDocument();
	});

	it('номер чата проверяется до отправки и уходит числом', async () => {
		await renderApp('/points/lenina/settings', { profile: managerProfile });
		server.profile = managerProfile;

		const field = await screen.findByRole('textbox', { name: 'Номер чата' });

		await userEvent.type(field, 'группа');
		await userEvent.click(screen.getByRole('button', { name: 'Подключить' }));
		expect(screen.getByText(/Нужен номер чата/)).toBeInTheDocument();
		expect(put).not.toHaveBeenCalled();

		await userEvent.clear(field);
		await userEvent.type(field, '-1001846203517');
		put.mockResolvedValueOnce({
			data: { ...lenina, telegram_chat_id: -1001846203517 },
		});
		await userEvent.click(screen.getByRole('button', { name: 'Подключить' }));

		expect(put).toHaveBeenCalledWith(
			'/points/lenina/telegram-chat',
			{ chat_id: -1001846203517 },
			expect.anything()
		);
		expect(await screen.findByText('подключены')).toBeInTheDocument();
	});

	it('у управляющего нет поля названия и отключения точки', async () => {
		await renderApp('/points/lenina/settings', { profile: managerProfile });

		expect(
			await screen.findByText('Переименовать и отключить точку может владелец')
		).toBeInTheDocument();
		expect(screen.queryByRole('textbox', { name: 'Название' })).toBeNull();
		expect(
			screen.queryByRole('button', { name: 'Отключить точку' })
		).toBeNull();
	});

	it('после подтверждения точка отключается и появляется плашка', async () => {
		await renderApp('/points/lenina/settings');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Отключить точку' })
		);
		const confirm = screen.getByRole('alertdialog', {
			name: 'Отключить «Точка на Ленина»?',
		});

		post.mockResolvedValueOnce({ data: { ...lenina, active: false } });
		server.profile = testProfile([testPoint({ active: false })]);
		await userEvent.click(
			within(confirm).getByRole('button', { name: 'Отключить точку' })
		);

		expect(post).toHaveBeenCalledWith(
			'/points/lenina/disable',
			undefined,
			expect.anything()
		);
		expect(await screen.findByText('Точка отключена.')).toBeInTheDocument();
	});
});

describe('доставка Мигом', () => {
	const migomCard = async () =>
		within(await screen.findByRole('region', { name: 'Доставка Мигом' }));

	it('владелец подключает заведение по номеру, не номер не отправляется', async () => {
		await renderApp('/points/lenina/settings');
		const card = await migomCard();

		expect(await card.findByText('не подключён')).toBeVisible();

		const field = card.getByRole('textbox', {
			name: 'Номер заведения в Мигоме',
		});

		await userEvent.type(field, '42а');
		await userEvent.click(card.getByRole('button', { name: 'Подключить' }));

		expect(
			card.getByText('Нужен номер заведения: целое число больше нуля')
		).toBeVisible();
		expect(put).not.toHaveBeenCalled();

		put.mockResolvedValueOnce({ data: { outlet_id: 42 } });
		await userEvent.clear(field);
		await userEvent.type(field, '42');
		await userEvent.click(card.getByRole('button', { name: 'Подключить' }));

		expect(put).toHaveBeenCalledWith('/points/lenina/migom', {
			outlet_id: 42,
		});
		expect(await card.findByText('заведение № 42')).toBeVisible();
		expect(card.getByText('подключён')).toBeVisible();
	});

	it('«Отключить Мигом» после подтверждения отправляет null', async () => {
		server.migom = { outlet_id: 42 };
		await renderApp('/points/lenina/settings');
		const card = await migomCard();

		await userEvent.click(
			await card.findByRole('button', { name: 'Отключить Мигом' })
		);
		const confirm = screen.getByRole('alertdialog', {
			name: 'Отключить Мигом от «Точка на Ленина»?',
		});

		put.mockResolvedValueOnce({ data: { outlet_id: null } });
		await userEvent.click(
			within(confirm).getByRole('button', { name: 'Отключить Мигом' })
		);

		expect(put).toHaveBeenCalledWith('/points/lenina/migom', {
			outlet_id: null,
		});
		expect(await card.findByText('не подключён')).toBeVisible();
		expect(
			card.getByRole('textbox', { name: 'Номер заведения в Мигоме' })
		).toHaveValue('');
	});

	it('на отключённой точке поля номера нет, Мигом можно только отключить', async () => {
		server.migom = { outlet_id: 42 };
		server.profile = testProfile([testPoint({ active: false })]);
		await renderApp('/points/lenina/settings', { profile: server.profile });
		const card = await migomCard();

		expect(await card.findByText('заведение № 42')).toBeVisible();
		expect(
			card.queryByRole('textbox', { name: 'Номер заведения в Мигоме' })
		).toBeNull();
		expect(card.getByRole('button', { name: 'Отключить Мигом' })).toBeVisible();
	});

	it('у управляющего карточки нет', async () => {
		await renderApp('/points/lenina/settings', { profile: managerProfile });

		await screen.findByText('Переименовать и отключить точку может владелец');
		expect(screen.queryByRole('region', { name: 'Доставка Мигом' })).toBeNull();
	});
});
