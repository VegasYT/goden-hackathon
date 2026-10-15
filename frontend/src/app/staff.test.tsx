import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Staff } from '@entities/organization';
import { ApiError, apiClient } from '@shared/api';

import { mockGet } from './test-api';
import {
	cook,
	manager,
	renderApp,
	testProfile,
	testSession,
} from './test-router';

const post = vi.spyOn(apiClient, 'post');
const put = vi.spyOn(apiClient, 'put');
const del = vi.spyOn(apiClient, 'delete');

const member = (user_id: string, name: string, role: Staff['role']): Staff => ({
	user_id,
	name,
	role,
	email: `${user_id}@mail.ru`,
});

const staff: Staff[] = [
	member('waiter', 'Сергей Иванов', 'waiter'),
	member('cook', 'Ольга Козлова', 'cook'),
	member('volkova', 'Наталья Волкова', 'manager'),
	member(testSession.userId, 'Илья Смирнов', 'manager'),
];

/** Состав точки на сервере: после записи тест задаёт новый. */
let onServer = staff;

const managerProfile = testProfile([manager]);

const cookProfile = testProfile([cook]);

const rowOf = (name: string) =>
	screen.getByRole('row', { name: new RegExp(name) });

beforeEach(() => {
	onServer = staff;
	mockGet(() => onServer);
});

describe('сотрудники точки', () => {
	it('показывает список по ролям; управляющий не трогает управляющих и себя', async () => {
		await renderApp('/points/lenina/staff', { profile: managerProfile });

		const table = await screen.findByRole('table', {
			name: 'Сотрудники точки',
		});
		const names = within(table)
			.getAllByRole('row')
			.slice(1)
			.map((row) => row.textContent);

		expect(names[0]).toMatch(/Илья Смирнов · это вы/);
		expect(names[1]).toMatch(/Наталья Волкова/);
		expect(names[2]).toMatch(/Ольга Козлова/);
		expect(names[3]).toMatch(/Сергей Иванов/);
		expect(
			within(rowOf('Наталья Волкова')).queryByRole('button', { name: /Снять/ })
		).toBeNull();
		expect(
			within(rowOf('Илья Смирнов')).queryByRole('button', { name: /Снять/ })
		).toBeNull();
		expect(
			within(rowOf('Ольга Козлова')).getByRole('button', {
				name: 'Снять с точки: Ольга Козлова',
			})
		).toBeInTheDocument();
	});

	it('повар видит список без действий и пояснение', async () => {
		await renderApp('/points/lenina/staff', { profile: cookProfile });

		await screen.findByRole('table', { name: 'Сотрудники точки' });
		expect(screen.getByRole('note')).toHaveTextContent(
			'Добавлять, снимать и менять роль могут управляющий и владелец'
		);
		expect(screen.queryByRole('button', { name: /Снять/ })).toBeNull();
		expect(
			screen.queryByRole('button', { name: 'Добавить сотрудника' })
		).toBeNull();
	});

	it('добавляет сотрудника по почте и роли', async () => {
		const belova = member('belova', 'Мария Белова', 'approver');

		post.mockResolvedValueOnce({ data: belova });
		await renderApp('/points/lenina/staff', { profile: managerProfile });

		await userEvent.click(
			await screen.findByRole('button', { name: 'Добавить сотрудника' })
		);
		const dialog = screen.getByRole('dialog', {
			name: 'Добавить сотрудника на точку',
		});

		expect(
			within(dialog).getByRole('radio', { name: /Управляющий/ })
		).toBeDisabled();
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Почта сотрудника' }),
			' belova@mail.ru '
		);
		await userEvent.click(
			within(dialog).getByRole('radio', { name: /Приёмщик/ })
		);
		onServer = [...staff, belova];
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Добавить' })
		);

		expect(post).toHaveBeenCalledWith(
			'/points/lenina/staff',
			{ email: 'belova@mail.ru', role: 'approver' },
			expect.anything()
		);
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(rowOf('Мария Белова')).toBeInTheDocument();
	});

	it('без почты запрос не уходит, на незнакомую почту — пояснение', async () => {
		await renderApp('/points/lenina/staff', { profile: managerProfile });

		await userEvent.click(
			await screen.findByRole('button', { name: 'Добавить сотрудника' })
		);
		const dialog = screen.getByRole('dialog');

		await userEvent.click(within(dialog).getByRole('radio', { name: /Повар/ }));
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Добавить' })
		);
		expect(
			within(dialog).getByText('Введите почту сотрудника')
		).toBeInTheDocument();
		expect(post).not.toHaveBeenCalled();

		post.mockRejectedValueOnce(
			new ApiError('пользователь не зарегистрирован', {
				status: 422,
				type: 'unknown-reference',
			})
		);
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Почта сотрудника' }),
			's.orlov@mail.ru'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Добавить' })
		);

		expect(
			await within(dialog).findByText(
				/Пользователь с почтой s\.orlov@mail\.ru ещё не входил в Goden/
			)
		).toBeInTheDocument();
	});

	it('смена роли уходит на сервер и показывает сообщение', async () => {
		put.mockResolvedValueOnce({
			data: member('cook', 'Ольга Козлова', 'waiter'),
		});
		await renderApp('/points/lenina/staff', { profile: managerProfile });

		await userEvent.click(
			await screen.findByRole('combobox', { name: 'Роль: Ольга Козлова' })
		);
		await userEvent.click(screen.getByRole('option', { name: 'Официант' }));

		expect(put).toHaveBeenCalledWith(
			'/points/lenina/staff/cook',
			{ role: 'waiter' },
			expect.anything()
		);
		expect(
			await screen.findByText('Ольга Козлова теперь официант')
		).toBeInTheDocument();
	});

	it('снимает сотрудника только после подтверждения', async () => {
		del.mockResolvedValueOnce({ data: undefined });
		await renderApp('/points/lenina/staff', { profile: managerProfile });

		await userEvent.click(
			await screen.findByRole('button', {
				name: 'Снять с точки: Ольга Козлова',
			})
		);
		await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));
		expect(del).not.toHaveBeenCalled();

		await userEvent.click(
			screen.getByRole('button', { name: 'Снять с точки: Ольга Козлова' })
		);
		const confirm = screen.getByRole('alertdialog', {
			name: 'Снять с точки: Ольга Козлова',
		});

		onServer = staff.filter(({ user_id }) => user_id !== 'cook');
		await userEvent.click(
			within(confirm).getByRole('button', { name: 'Снять с точки' })
		);

		expect(del).toHaveBeenCalledWith(
			'/points/lenina/staff/cook',
			expect.anything()
		);
		await waitFor(() =>
			expect(screen.queryByRole('row', { name: /Ольга Козлова/ })).toBeNull()
		);
	});
});
