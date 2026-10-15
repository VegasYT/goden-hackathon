import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { writeLastPoint } from '@entities/profile';

import {
	cook,
	manager,
	navSections,
	renderApp,
	testPoint,
	testProfile,
	testSession,
} from './test-router';

const cookProfile = testProfile([cook]);
const managerProfile = testProfile([manager]);

describe('роуты', () => {
	it('неизвестный путь показывает страницу «не найдено»', async () => {
		await renderApp('/no-such-page');

		expect(
			await screen.findByRole('heading', { name: 'Страница не найдена' })
		).toBeInTheDocument();
	});

	it('в режиме разработки /dev/ui показывает витрину', async () => {
		await renderApp('/dev/ui');

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Витрина дизайн-системы',
			})
		).toBeInTheDocument();
	});

	it('адрес / открывает последнюю выбранную точку', async () => {
		const profile = testProfile([
			testPoint(),
			testPoint({ id: 'mira', name: 'Точка на Мира' }),
		]);

		writeLastPoint(testSession.userId, 'mira');
		const { router } = await renderApp('/', { profile });

		expect(router.state.location.pathname).toBe('/points/mira');
	});

	it('если последней точки нет в профиле, адрес / открывает первую', async () => {
		writeLastPoint(testSession.userId, 'gone');
		const { router } = await renderApp('/');

		expect(router.state.location.pathname).toBe('/points/lenina');
	});

	it('без точек в профиле адрес / ведёт на первый вход', async () => {
		const { router } = await renderApp('/', { profile: testProfile([], []) });

		expect(router.state.location.pathname).toBe('/welcome');
	});

	it('точка не из профиля показывает «Точка недоступна» без разделов', async () => {
		await renderApp('/points/foreign/stock');

		expect(
			await screen.findByRole('heading', { name: 'Точка недоступна' })
		).toBeInTheDocument();
		expect(navSections()).toEqual([]);
		expect(screen.queryByRole('heading', { name: 'Склад' })).toBeNull();
	});

	it('повару в навигации доступны только сводка, кухня, стоп-лист и сотрудники', async () => {
		await renderApp('/points/lenina', { profile: cookProfile });

		await screen.findByRole('navigation', { name: 'Основная навигация' });
		expect(navSections()).toEqual([
			'Сводка',
			'Кухня',
			'Стоп-лист',
			'Сотрудники',
		]);
	});

	it('управляющему видны «Сотрудники» и «Настройки точки», но не группа компании', async () => {
		await renderApp('/points/lenina', { profile: managerProfile });

		const nav = await screen.findByRole('navigation', {
			name: 'Основная навигация',
		});

		expect(navSections()).toEqual([
			'Сводка',
			'Склад',
			'Поставки',
			'Заказы',
			'Кухня',
			'Стоп-лист',
			'Аналитика',
			'Сотрудники',
			'Настройки точки',
		]);
		expect(within(nav).queryByText(/^Компания «/)).toBeNull();
	});

	it('владельцу после разделов точки видна группа его компании', async () => {
		await renderApp('/points/lenina');

		const nav = await screen.findByRole('navigation', {
			name: 'Основная навигация',
		});

		expect(within(nav).getByText('Компания «Демо»')).toBeInTheDocument();
		expect(navSections().slice(-4)).toEqual([
			'Настройки точки',
			'Каталог',
			'Точки и компания',
			'Генератор',
		]);
	});

	it.each(['company', 'catalog'])(
		'раздел компании %s управляющему недоступен',
		async (section) => {
			await renderApp(`/points/lenina/${section}`, { profile: managerProfile });

			expect(
				await screen.findByRole('heading', { name: 'Нет доступа к разделу' })
			).toBeInTheDocument();
		}
	);

	it('раздел без права показывает «Нет доступа к разделу» внутри оболочки', async () => {
		await renderApp('/points/lenina/stock', { profile: cookProfile });

		expect(
			await screen.findByRole('heading', { name: 'Нет доступа к разделу' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('heading', { level: 1, name: 'Склад' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('navigation', { name: 'Основная навигация' })
		).toBeInTheDocument();
	});

	it('пункты навигации ведут на разделы выбранной точки', async () => {
		const profile = testProfile([
			testPoint(),
			testPoint({ id: 'mira', name: 'Точка на Мира' }),
		]);

		await renderApp('/points/mira', { profile });

		expect(await screen.findByRole('link', { name: 'Склад' })).toHaveAttribute(
			'href',
			'/points/mira/stock'
		);
	});
});
