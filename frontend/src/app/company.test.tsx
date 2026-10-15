import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Point } from '@entities/organization';
import type { Profile } from '@entities/profile';
import { ApiError, apiClient, type components } from '@shared/api';

import { mockGet } from './test-api';
import {
	renderApp,
	testOrgPoint,
	testPoint,
	testProfile,
	testSession,
} from './test-router';

const post = vi.spyOn(apiClient, 'post');
const put = vi.spyOn(apiClient, 'put');
const patch = vi.spyOn(apiClient, 'patch');
const del = vi.spyOn(apiClient, 'delete');

type Supplier = components['schemas']['Supplier'];

const point = (id: string, name: string, active = true) =>
	testOrgPoint({ id, name, active });

const demo = {
	id: 'demo',
	name: 'Демо',
	owner_id: testSession.userId,
	active: true,
};
const cafe = {
	id: 'cafe',
	name: 'Кафе у моста',
	owner_id: testSession.userId,
	active: true,
};

const lenina = testPoint();
const vokzal = testPoint({
	id: 'vokzal',
	name: 'Точка у вокзала',
	active: false,
});
const center = testPoint({
	id: 'center',
	company_id: 'cafe',
	company_name: 'Кафе у моста',
	name: 'Центр',
});

const supplier = (overrides: Partial<Supplier> = {}): Supplier => ({
	id: 'ug',
	company_id: 'demo',
	name: 'Овощебаза Юг',
	point_ids: ['lenina'],
	key_active: true,
	created_at: '2026-10-15T09:00:00Z',
	...overrides,
});

/** Профиль, точки и поставщики компании на сервере: после записи тест задаёт новые. */
let server: { profile: Profile; points: Point[]; suppliers: Supplier[] };

beforeEach(() => {
	server = {
		profile: testProfile([lenina, vokzal, center], [demo, cafe]),
		points: [
			point('lenina', 'Точка на Ленина'),
			point('vokzal', 'Точка у вокзала', false),
		],
		suppliers: [],
	};
	mockGet((url) =>
		url === '/me'
			? server.profile
			: url.endsWith('/suppliers')
				? server.suppliers
				: server.points
	);
});

describe('точки и компания', () => {
	it('показывает точки компании выбранной точки, отключённую с пометкой', async () => {
		await renderApp('/points/lenina/company');

		const table = await screen.findByRole('table', { name: 'Точки компании' });
		const vokzalRow = within(table).getByRole('row', {
			name: /Точка у вокзала/,
		});

		expect(within(vokzalRow).getByText('отключена')).toBeInTheDocument();
		expect(
			within(table).getByRole('link', { name: 'Настройки: Точка на Ленина' })
		).toHaveAttribute('href', '/points/lenina/settings');
	});

	it('новая точка появляется в переключателе', async () => {
		await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Новая точка' })
		);
		const dialog = screen.getByRole('dialog', {
			name: 'Новая точка в компании «Демо»',
		});

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название точки' }),
			'Мира'
		);
		post.mockResolvedValueOnce({ data: point('mira', 'Мира') });
		server.profile = testProfile(
			[lenina, testPoint({ id: 'mira', name: 'Мира' }), vokzal, center],
			[demo, cafe]
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать точку' })
		);

		expect(post).toHaveBeenCalledWith(
			'/companies/demo/points',
			{ name: 'Мира' },
			expect.anything()
		);
		expect(await screen.findByText('Точка «Мира» создана')).toBeInTheDocument();
		await userEvent.click(
			screen.getByRole('button', { name: /^Точка: Точка на Ленина/ })
		);
		expect(
			screen.getByRole('menuitemradio', { name: /Мира/ })
		).toBeInTheDocument();
	});

	it('сообщение о новой точке с «Открыть» не закрывается щелчком мимо', async () => {
		await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Новая точка' })
		);
		const dialog = screen.getByRole('dialog', {
			name: 'Новая точка в компании «Демо»',
		});

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название точки' }),
			'Мира'
		);
		post.mockResolvedValueOnce({ data: point('mira', 'Мира') });
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать точку' })
		);
		expect(await screen.findByText('Точка «Мира» создана')).toBeInTheDocument();

		await userEvent.click(
			screen.getByRole('heading', { name: 'Точки и компания' })
		);

		await expect(
			waitFor(() =>
				expect(screen.queryByText('Точка «Мира» создана')).toBeNull()
			)
		).rejects.toThrow();
		expect(screen.getByRole('link', { name: 'Открыть' })).toHaveAttribute(
			'href',
			'/points/mira'
		);
	});

	it('новое название компании видно в подписи группы навигации', async () => {
		await renderApp('/points/lenina/company');

		const field = await screen.findByRole('textbox', { name: 'Название' });

		await userEvent.clear(field);
		await userEvent.type(field, 'Флагман');
		patch.mockResolvedValueOnce({ data: { ...demo, name: 'Флагман' } });
		server.profile = testProfile(
			[
				{ ...lenina, company_name: 'Флагман' },
				{ ...vokzal, company_name: 'Флагман' },
				center,
			],
			[{ ...demo, name: 'Флагман' }, cafe]
		);
		await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

		expect(await screen.findByText('Компания «Флагман»')).toBeInTheDocument();
	});

	it('передача нужна с отметкой согласия и уводит на точку другой компании', async () => {
		const { router } = await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Передать компанию' })
		);
		const dialog = screen.getByRole('dialog', {
			name: 'Передать компанию «Демо»',
		});
		const confirm = within(dialog).getByRole('button', {
			name: 'Передать компанию',
		});

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Почта нового владельца' }),
			'p.sokolov@mail.ru'
		);
		expect(confirm).toBeDisabled();

		await userEvent.click(
			within(dialog).getByRole('checkbox', {
				name: 'Понимаю, что потеряю доступ',
			})
		);
		put.mockResolvedValueOnce({ data: { ...demo, owner_id: 'sokolov' } });
		server.profile = testProfile([center], [cafe]);
		await userEvent.click(confirm);

		expect(put).toHaveBeenCalledWith(
			'/companies/demo/owner',
			{ email: 'p.sokolov@mail.ru' },
			expect.anything()
		);
		await waitFor(() =>
			expect(router.state.location.pathname).toBe('/points/center')
		);
	});

	it('отключение компании подтверждается её названием', async () => {
		await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Отключить компанию' })
		);
		const dialog = screen.getByRole('alertdialog', {
			name: 'Отключить компанию «Демо»?',
		});
		const confirm = within(dialog).getByRole('button', {
			name: 'Отключить компанию',
		});
		const field = within(dialog).getByRole('textbox', {
			name: 'Введите «Демо», чтобы подтвердить',
		});

		await userEvent.type(field, 'Дем');
		expect(confirm).toBeDisabled();

		await userEvent.type(field, 'о');
		post.mockResolvedValueOnce({ data: { ...demo, active: false } });
		server.profile = testProfile(
			[lenina, vokzal, center],
			[{ ...demo, active: false }, cafe]
		);
		await userEvent.click(confirm);

		expect(post).toHaveBeenCalledWith(
			'/companies/demo/disable',
			undefined,
			expect.anything()
		);
		const table = await screen.findByRole('table', { name: 'Точки компании' });

		await waitFor(() =>
			expect(within(table).getAllByText('отключена')).toHaveLength(2)
		);
		expect(screen.queryByRole('button', { name: 'Новая точка' })).toBeNull();
	});
});

describe('поставщики', () => {
	it('показывает поставщиков с точками и состоянием ключа', async () => {
		server.suppliers = [
			supplier(),
			supplier({ id: 'milk', name: 'Молочная ферма', key_active: false }),
		];
		await renderApp('/points/lenina/company');

		const table = await screen.findByRole('table', {
			name: 'Поставщики компании',
		});
		const ug = within(table).getByRole('row', { name: /Овощебаза Юг/ });
		const milk = within(table).getByRole('row', { name: /Молочная ферма/ });

		expect(within(ug).getByText('Точка на Ленина')).toBeInTheDocument();
		expect(within(ug).getByText('ключ действует')).toBeInTheDocument();
		expect(within(milk).getByText('ключ отозван')).toBeInTheDocument();
	});

	it('без поставщиков объясняет, зачем они нужны', async () => {
		await renderApp('/points/lenina/company');

		expect(await screen.findByText('Поставщиков пока нет')).toBeInTheDocument();
		expect(
			screen.getByText(/сам регистрирует поставки на своих точках/)
		).toBeInTheDocument();
	});

	it('у отключённой компании поставщиков не меняют', async () => {
		server.profile = testProfile(
			[lenina, vokzal, center],
			[{ ...demo, active: false }, cafe]
		);
		server.suppliers = [supplier()];
		await renderApp('/points/lenina/company', { profile: server.profile });

		expect(
			await screen.findByRole('row', { name: /Овощебаза Юг/ })
		).toBeInTheDocument();
		expect(
			screen.queryByRole('button', { name: 'Новый поставщик' })
		).toBeNull();
		expect(
			screen.queryByRole('button', { name: 'Действия: Овощебаза Юг' })
		).toBeNull();
	});

	it('новый поставщик: без точки запрос не уходит, после создания показан ключ, Esc окно ключа не закрывает', async () => {
		await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Новый поставщик' })
		);
		const dialog = screen.getByRole('dialog', { name: 'Новый поставщик' });

		expect(
			within(dialog).queryByRole('checkbox', { name: 'Точка у вокзала' })
		).toBeNull();
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название поставщика' }),
			'Овощебаза Юг'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать' })
		);

		expect(post).not.toHaveBeenCalled();
		expect(
			within(dialog).getByText('Отметьте хотя бы одну точку')
		).toBeInTheDocument();

		await userEvent.click(
			within(dialog).getByRole('checkbox', { name: 'Точка на Ленина' })
		);
		post.mockResolvedValueOnce({ data: supplier({ key: 'gdn_secret' }) });
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать' })
		);

		expect(post).toHaveBeenCalledWith('/companies/demo/suppliers', {
			name: 'Овощебаза Юг',
			point_ids: ['lenina'],
		});
		const keyDialog = await screen.findByRole('dialog', {
			name: 'Ключ поставщика «Овощебаза Юг»',
		});

		expect(
			within(keyDialog).getByRole('textbox', { name: 'Ключ' })
		).toHaveValue('gdn_secret');
		expect(
			within(keyDialog).getByRole('textbox', {
				name: 'Точка «Точка на Ленина»',
			})
		).toHaveValue('lenina');

		await userEvent.keyboard('{Escape}');

		expect(keyDialog).toBeInTheDocument();

		await userEvent.click(
			within(keyDialog).getByRole('button', { name: 'Готово' })
		);

		expect(
			await screen.findByRole('row', { name: /Овощебаза Юг/ })
		).toBeInTheDocument();
		expect(screen.queryByDisplayValue('gdn_secret')).toBeNull();
	});

	it('занятое название показано у поля, введённое сохраняется', async () => {
		await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Новый поставщик' })
		);
		const dialog = screen.getByRole('dialog', { name: 'Новый поставщик' });
		const name = within(dialog).getByRole('textbox', {
			name: 'Название поставщика',
		});

		await userEvent.type(name, 'Овощебаза Юг');
		await userEvent.click(
			within(dialog).getByRole('checkbox', { name: 'Точка на Ленина' })
		);
		post.mockRejectedValueOnce(
			new ApiError('Название уже занято', {
				status: 409,
				fieldErrors: {
					name: { message: 'название уже занято', code: 'taken' },
				},
			})
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать' })
		);

		expect(
			await within(dialog).findByText('название уже занято')
		).toBeInTheDocument();
		expect(name).toHaveValue('Овощебаза Юг');
	});

	it('перевыпуск показывает новый ключ, после отзыва есть «Выпустить ключ»', async () => {
		server.suppliers = [supplier()];
		await renderApp('/points/lenina/company');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Действия: Овощебаза Юг' })
		);
		await userEvent.click(
			screen.getByRole('menuitem', { name: 'Перевыпустить ключ' })
		);
		post.mockResolvedValueOnce({ data: supplier({ key: 'gdn_new' }) });
		await userEvent.click(
			within(
				screen.getByRole('alertdialog', {
					name: 'Перевыпустить ключ поставщика «Овощебаза Юг»?',
				})
			).getByRole('button', { name: 'Перевыпустить ключ' })
		);

		expect(post).toHaveBeenCalledWith('/companies/demo/suppliers/ug/key');
		const keyDialog = await screen.findByRole('dialog', {
			name: 'Ключ поставщика «Овощебаза Юг»',
		});

		expect(
			within(keyDialog).getByRole('textbox', { name: 'Ключ' })
		).toHaveValue('gdn_new');
		await userEvent.click(
			within(keyDialog).getByRole('button', { name: 'Готово' })
		);

		await userEvent.click(
			await screen.findByRole('button', { name: 'Действия: Овощебаза Юг' })
		);
		await userEvent.click(
			screen.getByRole('menuitem', { name: 'Отозвать ключ' })
		);
		del.mockResolvedValueOnce({ data: supplier({ key_active: false }) });
		await userEvent.click(
			within(
				screen.getByRole('alertdialog', {
					name: 'Отозвать ключ поставщика «Овощебаза Юг»?',
				})
			).getByRole('button', { name: 'Отозвать ключ' })
		);

		expect(del).toHaveBeenCalledWith('/companies/demo/suppliers/ug/key');
		expect(await screen.findByText('ключ отозван')).toBeInTheDocument();
		// Пока диалог закрывается, остальная страница скрыта от чтения с экрана.
		await userEvent.click(
			await screen.findByRole('button', { name: 'Действия: Овощебаза Юг' })
		);
		expect(
			screen.getByRole('menuitem', { name: 'Выпустить ключ' })
		).toBeInTheDocument();
		expect(
			screen.queryByRole('menuitem', { name: 'Отозвать ключ' })
		).toBeNull();
	});
});
