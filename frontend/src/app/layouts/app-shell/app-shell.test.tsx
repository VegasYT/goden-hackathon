import { createRootRoute, createRoute, Outlet } from '@tanstack/react-router';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { logout } from '@shared/auth';
import { mockMatchMedia } from '@shared/lib/test-utils';
import type { ConnectionState } from '@shared/ui/connection-status';

import { renderApp, testPoint, testProfile } from '../../test-router';

import { AppShell } from './app-shell';

vi.mock('@shared/auth', async (importOriginal) => ({
	...(await importOriginal<typeof import('@shared/auth')>()),
	logout: vi.fn(() => Promise.resolve()),
}));

async function renderShell(connection?: ConnectionState) {
	const root = createRootRoute({
		component: () => (
			<AppShell
				profile={testProfile()}
				point={testPoint()}
				connection={connection}
			>
				<Outlet />
			</AppShell>
		),
	});
	const index = createRoute({ getParentRoute: () => root, path: '/' });

	await renderApp('/', { tree: root.addChildren([index]) });
}

describe('AppShell', () => {
	it('отмечает текущий раздел как текущую страницу', async () => {
		await renderApp('/points/lenina/stock');

		expect(await screen.findByRole('link', { name: 'Склад' })).toHaveAttribute(
			'aria-current',
			'page'
		);
		expect(screen.getByRole('link', { name: 'Сводка' })).not.toHaveAttribute(
			'aria-current'
		);
	});

	it('с ширины 1200px показывает боковую панель с логотипом-ссылкой на сводку', async () => {
		await renderApp('/points/lenina/orders');

		expect(await screen.findByRole('link', { name: 'Goden' })).toHaveAttribute(
			'href',
			'/points/lenina'
		);
	});

	it('уже 1200px показывает рельс со знаком-ссылкой на сводку', async () => {
		mockMatchMedia(() => false);
		await renderApp('/points/lenina/orders', { wide: false });

		expect(
			await screen.findByRole('link', { name: 'Goden — на сводку' })
		).toHaveAttribute('href', '/points/lenina');
	});

	it('без источника связи нет ни индикатора, ни плашки', async () => {
		await renderShell();

		expect(screen.queryByRole('status')).toBeNull();
		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('при потере связи показывает плашку', async () => {
		await renderShell({
			status: 'offline',
			dataAt: new Date(2026, 9, 10, 14, 32),
			onRetry: () => {},
		});

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Нет связи с сервером. Данные на 14:32'
		);
	});

	it('у владельца в профиле подпись «Владелец», а «Выйти» в меню завершает сессию', async () => {
		await renderShell();

		await userEvent.click(
			await screen.findByRole('button', {
				name: 'Профиль: Менеджер Тестовый, Владелец',
			})
		);
		await userEvent.click(screen.getByRole('menuitem', { name: 'Выйти' }));
		expect(logout).toHaveBeenCalled();
	});

	it('у сотрудника в профиле его роль на выбранной точке', async () => {
		await renderApp('/points/lenina', {
			profile: testProfile([testPoint({ owner: false, role: 'manager' })]),
		});

		expect(
			await screen.findByRole('button', {
				name: 'Профиль: Менеджер Тестовый, Управляющий',
			})
		).toBeInTheDocument();
	});

	it('на отключённой точке показывает плашку «Точка отключена»', async () => {
		await renderApp('/points/lenina', {
			profile: testProfile([testPoint({ active: false })]),
		});

		expect(await screen.findByText('Точка отключена.')).toBeInTheDocument();
	});
});
