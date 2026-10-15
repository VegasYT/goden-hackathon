import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Dish } from '@entities/dish';
import type { Product } from '@entities/product';
import type { Profile } from '@entities/profile';
import { apiClient, ApiError } from '@shared/api';
import { retype } from '@shared/lib/test-utils';

import { lastListParams, mockGet } from './test-api';
import { testProduct } from './test-stock';
import { renderApp, testPoint, testProfile, testSession } from './test-router';

const post = vi.spyOn(apiClient, 'post');
const put = vi.spyOn(apiClient, 'put');
const patch = vi.spyOn(apiClient, 'patch');

const product = (id: string, name: string, overrides: Partial<Product> = {}) =>
	testProduct({
		id,
		name,
		category: 'Молочные продукты',
		shelf_life_days: 7,
		dishes: 0,
		unit_locked: false,
		...overrides,
	});

const bacon = product('bacon', 'Бекон', {
	category: 'Мясо',
	dishes: 1,
	unit_locked: true,
});
const parmesan = product('parmesan', 'Сыр пармезан', { dishes: 1 });
const mozzarella = product('mozzarella', 'Сыр моцарелла', {
	unit_locked: true,
});
const salmon = product('salmon', 'Лосось охлаждённый', {
	category: 'Рыба',
	shelf_life_days: 4,
	archived: true,
});

const ingredient = (item: Product, quantity: number) => ({
	product_id: item.id,
	product_name: item.name,
	unit: item.unit,
	quantity,
	product_archived: item.archived,
});

const carbonara: Dish = {
	id: 'carbonara',
	company_id: 'demo',
	name: 'Паста карбонара',
	archived: false,
	ingredients: [ingredient(bacon, 60), ingredient(parmesan, 20)],
};
const soup: Dish = {
	id: 'soup',
	company_id: 'demo',
	name: 'Суп томатный',
	archived: false,
	ingredients: [],
};
const carpaccio: Dish = {
	id: 'carpaccio',
	company_id: 'demo',
	name: 'Карпаччо из лосося',
	archived: true,
	ingredients: [ingredient(salmon, 80)],
};

let server: {
	products: Product[];
	dishes: Dish[];
	total?: number;
	profile: Profile;
};

const page = <T,>(items: T[]) => ({
	items,
	total: server.total ?? items.length,
});

const matches = (name: string, q: unknown) =>
	!q || name.toLowerCase().includes(String(q).toLowerCase());

function catalogApi() {
	return mockGet((url, params) => {
		if (url === '/me') {
			return server.profile;
		}

		if (url.endsWith('/product-categories')) {
			return {
				items: [
					{ name: 'Молочные продукты', products: 2 },
					{ name: 'Мясо', products: 1 },
				],
			};
		}

		const productId = /\/products\/([^/]+)$/.exec(url)?.[1];

		if (productId) {
			const found = server.products.find(({ id }) => id === productId);

			return {
				...found,
				used_in: server.dishes
					.filter(
						(dish) =>
							!dish.archived &&
							dish.ingredients.some((item) => item.product_id === productId)
					)
					.map(({ id, name }) => ({ id, name })),
			};
		}

		if (url.endsWith('/products')) {
			const archived = params.status === 'archived';

			return page(
				server.products.filter(
					(item) => item.archived === archived && matches(item.name, params.q)
				)
			);
		}

		const dishId = /\/dishes\/([^/]+)$/.exec(url)?.[1];

		if (dishId) {
			return server.dishes.find(({ id }) => id === dishId);
		}

		if (url.endsWith('/dishes')) {
			const archived = params.status === 'archived';

			return page(
				server.dishes.filter(
					(dish) =>
						dish.archived === archived &&
						(params.recipe !== 'missing' || dish.ingredients.length === 0) &&
						matches(dish.name, params.q)
				)
			);
		}

		return { items: [], total: 0 };
	});
}

let get: ReturnType<typeof catalogApi>;

beforeEach(() => {
	server = {
		products: [bacon, parmesan, mozzarella, salmon],
		dishes: [carbonara, soup, carpaccio],
		profile: testProfile(),
	};
	get = catalogApi();
});

const productsTable = () =>
	screen.findByRole('table', { name: 'Продукты каталога' });

describe('раздел каталога', () => {
	it('владелец видит каталог компании с числами на вкладках', async () => {
		await renderApp('/points/lenina/catalog');

		expect(
			await screen.findByText('Общий для всех точек компании «Демо»')
		).toBeInTheDocument();
		expect(
			within(await productsTable()).getByRole('row', { name: /Бекон/ })
		).toHaveTextContent('1 блюдо');
		expect(screen.getByRole('tab', { name: /Продукты\s*3/ })).toHaveAttribute(
			'aria-selected',
			'true'
		);
		expect(
			await screen.findByRole('tab', { name: /Блюда\s*2/ })
		).toBeInTheDocument();
	});

	it('на точке другой компании показывает её каталог', async () => {
		const profile = testProfile([
			testPoint(),
			testPoint({
				id: 'center',
				company_id: 'cafe',
				company_name: 'Кафе у моста',
			}),
		]);

		server.profile = profile;

		await renderApp('/points/center/catalog', { profile });

		await screen.findByText('Общий для всех точек компании «Кафе у моста»');
		await waitFor(() =>
			expect(lastListParams(get, '/companies/cafe/products')).toBeDefined()
		);
	});

	it('на отключённой компании каталог только для чтения', async () => {
		const profile = testProfile(undefined, [
			{
				id: 'demo',
				name: 'Демо',
				owner_id: testSession.userId,
				active: false,
			},
		]);

		server.profile = profile;
		await renderApp('/points/lenina/catalog', { profile });

		expect(
			await screen.findByText(
				'Компания отключена — каталог доступен только для чтения'
			)
		).toBeInTheDocument();
		await productsTable();
		expect(screen.queryByRole('button', { name: 'Новый продукт' })).toBeNull();
		expect(screen.queryByRole('button', { name: /^Действия:/ })).toBeNull();
	});
});

describe('список продуктов', () => {
	it('поиск и сортировка по категории уходят на сервер и в адрес', async () => {
		const { router } = await renderApp('/points/lenina/catalog');

		await productsTable();
		await userEvent.type(
			screen.getByRole('textbox', { name: 'Поиск по продукту' }),
			'сыр'
		);
		await waitFor(() =>
			expect(lastListParams(get, '/products')).toMatchObject({ q: 'сыр' })
		);
		await userEvent.click(screen.getByRole('button', { name: 'Категория' }));

		await waitFor(() =>
			expect(lastListParams(get, '/products')).toMatchObject({
				q: 'сыр',
				sort: 'category',
			})
		);
		expect(router.state.location.search).toEqual({
			q: 'сыр',
			sort: 'category',
		});
	});

	it('заголовок колонки сортировки сообщает её направление', async () => {
		await renderApp('/points/lenina/catalog');

		const table = await productsTable();
		const header = (name: string) =>
			within(table).getByRole('columnheader', { name });

		expect(header('Продукт')).toHaveAttribute('aria-sort', 'ascending');
		expect(header('Категория')).not.toHaveAttribute('aria-sort');

		await userEvent.click(
			within(table).getByRole('button', { name: 'Категория' })
		);

		await waitFor(() =>
			expect(header('Категория')).toHaveAttribute('aria-sort', 'ascending')
		);
		expect(header('Продукт')).not.toHaveAttribute('aria-sort');
	});

	it('смена категории возвращает на первую страницу', async () => {
		server.total = 30;
		const { router } = await renderApp('/points/lenina/catalog?page=2');

		await productsTable();
		await userEvent.click(screen.getByRole('combobox', { name: 'Категория' }));
		await userEvent.click(
			await screen.findByRole('option', { name: 'Молочные продукты' })
		);

		await waitFor(() =>
			expect(router.state.location.search).toEqual({
				category: 'Молочные продукты',
			})
		);
		expect(lastListParams(get, '/products')).toMatchObject({
			category: 'Молочные продукты',
			offset: 0,
		});
	});

	it('пустой каталог предлагает создать продукт', async () => {
		server.products = [];
		await renderApp('/points/lenina/catalog');

		expect(await screen.findByText('Каталог пока пустой')).toBeVisible();
		expect(
			screen.getAllByRole('button', { name: 'Новый продукт' })
		).toHaveLength(2);
	});
});

describe('диалог продукта', () => {
	it('новый продукт отправляет название, единицу, новую категорию и срок', async () => {
		post.mockResolvedValueOnce({
			data: product('tuna', 'Тунец', { category: 'Рыба и морепродукты' }),
		});
		await renderApp('/points/lenina/catalog');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Новый продукт' })
		);
		const dialog = await screen.findByRole('dialog', { name: 'Новый продукт' });

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название' }),
			'Тунец'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'штуки' })
		);
		await userEvent.type(
			within(dialog).getByRole('combobox', { name: 'Категория' }),
			'Рыба и морепродукты'
		);
		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Срок хранения, дней' }),
			'4'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать продукт' })
		);

		await waitFor(() =>
			expect(post).toHaveBeenCalledWith(
				'/companies/demo/products',
				{
					name: 'Тунец',
					unit: 'pcs',
					category: 'Рыба и морепродукты',
					shelf_life_days: 4,
				},
				expect.anything()
			)
		);
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	});

	it('«Без срока» снимает срок, единица использованного продукта заблокирована', async () => {
		patch.mockResolvedValueOnce({
			data: { ...mozzarella, shelf_life_days: undefined },
		});
		await renderApp('/points/lenina/catalog');

		await userEvent.click(
			within(await productsTable()).getByRole('button', {
				name: 'Сыр моцарелла',
			})
		);
		const dialog = await screen.findByRole('dialog', {
			name: 'Изменить продукт',
		});

		expect(
			within(dialog).getByRole('button', { name: 'граммы' })
		).toBeDisabled();
		expect(
			within(dialog).getByText(
				'Единицу нельзя сменить: продукт уже есть в поставках и рецептурах'
			)
		).toBeVisible();

		await userEvent.click(
			within(dialog).getByRole('checkbox', { name: 'Без срока' })
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Сохранить' })
		);

		await waitFor(() =>
			expect(patch).toHaveBeenCalledWith(
				'/companies/demo/products/mozzarella',
				{ shelf_life_days: null },
				expect.anything()
			)
		);
	});

	it('название продукта в архиве показывается у поля со ссылкой на архив', async () => {
		post.mockRejectedValueOnce(
			new ApiError('Название занято', {
				status: 409,
				fieldErrors: {
					name: { message: 'Название занято', code: 'taken_by_archived' },
				},
			})
		);
		await renderApp('/points/lenina/catalog');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Новый продукт' })
		);
		const dialog = await screen.findByRole('dialog', { name: 'Новый продукт' });

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название' }),
			'Лосось охлаждённый'
		);
		await userEvent.click(
			within(dialog).getByRole('checkbox', { name: 'Без срока' })
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать продукт' })
		);

		expect(
			await within(dialog).findByText(/Это название у продукта в архиве/)
		).toBeVisible();
		expect(
			within(dialog).getByRole('link', { name: 'открыть архив' })
		).toHaveAttribute('href', expect.stringMatching(/archived=true.*q=/));
	});
});

describe('архив продуктов', () => {
	it('продукт в рецептуре не отправляется в архив, отказ называет блюдо', async () => {
		await renderApp('/points/lenina/catalog');

		await userEvent.click(
			within(await productsTable()).getByRole('button', {
				name: 'Действия: Бекон',
			})
		);
		await userEvent.click(
			await screen.findByRole('menuitem', { name: 'Отправить в архив' })
		);

		const dialog = await screen.findByRole('dialog', {
			name: 'Нельзя отправить в архив',
		});

		expect(await within(dialog).findByText('Паста карбонара')).toBeVisible();
		expect(post).not.toHaveBeenCalled();
	});

	it('восстановленный продукт пропадает из архива', async () => {
		post.mockImplementationOnce(async () => {
			server.products = server.products.map((item) =>
				item.id === 'salmon' ? { ...item, archived: false } : item
			);

			return { data: { ...salmon, archived: false } };
		});
		await renderApp('/points/lenina/catalog?archived=true');

		await userEvent.click(
			within(await productsTable()).getByRole('button', {
				name: 'Восстановить: Лосось охлаждённый',
			})
		);

		expect(post).toHaveBeenCalledWith(
			'/companies/demo/products/salmon/restore',
			undefined,
			expect.anything()
		);
		expect(
			await screen.findByText('«Лосось охлаждённый» снова в каталоге')
		).toBeVisible();
		await waitFor(() =>
			expect(
				screen.queryByRole('table', { name: 'Продукты каталога' })
			).toBeNull()
		);
	});
});

const dishesTable = () =>
	screen.findByRole('table', { name: 'Блюда каталога' });

describe('список блюд', () => {
	it('рецептура строкой, «Без рецептуры» уходит на сервер', async () => {
		await renderApp('/points/lenina/catalog/dishes');

		expect(
			within(await dishesTable()).getByRole('row', { name: /Паста карбонара/ })
		).toHaveTextContent(/Бекон 60\sг · Сыр пармезан 20\sг/);

		await userEvent.click(
			screen.getByRole('button', { name: /^Без рецептуры/ })
		);

		await waitFor(() =>
			expect(lastListParams(get, '/dishes')).toMatchObject({
				recipe: 'missing',
			})
		);
		expect(
			within(await dishesTable()).getByText(
				'нет рецептуры — блюдо в стоп-листе'
			)
		).toBeVisible();
	});
});

async function openCarbonara() {
	const view = await renderApp('/points/lenina/catalog/dishes?dish=carbonara');
	const panel = await screen.findByRole('dialog', { name: 'Паста карбонара' });

	await within(panel).findByRole('textbox', { name: /^Количество: Бекон/ });

	return { ...view, panel };
}

async function searchProduct(scope: HTMLElement, text: string) {
	await userEvent.type(
		within(scope).getByRole('combobox', { name: 'Добавить продукт' }),
		text
	);
}

const setQuantity = (scope: HTMLElement, name: string, value: string) =>
	retype(
		within(scope).getByRole('textbox', {
			name: new RegExp(`^Количество: ${name}`),
		}),
		value
	);

describe('рецептура блюда', () => {
	it('добавленный продукт сохраняется вместе со всей рецептурой', async () => {
		put.mockResolvedValueOnce({ data: carbonara });
		const { panel } = await openCarbonara();

		await searchProduct(panel, 'сыр');

		expect(
			await screen.findByRole('option', { name: /Сыр пармезан/ })
		).toHaveAttribute('aria-disabled', 'true');
		expect(
			screen.getByRole('option', { name: /Сыр пармезан/ })
		).toHaveTextContent('уже в рецептуре');

		await userEvent.click(
			screen.getByRole('option', { name: /Сыр моцарелла/ })
		);
		await setQuantity(panel, 'Сыр моцарелла', '30');
		await userEvent.click(
			within(panel).getByRole('button', { name: 'Сохранить рецептуру' })
		);

		await waitFor(() =>
			expect(put).toHaveBeenCalledWith(
				'/companies/demo/dishes/carbonara/recipe',
				{
					ingredients: [
						{ product_id: 'bacon', quantity: 60 },
						{ product_id: 'parmesan', quantity: 20 },
						{ product_id: 'mozzarella', quantity: 30 },
					],
				},
				expect.anything()
			)
		);
	});

	it('новый продукт из поиска создаётся и попадает в рецептуру', async () => {
		post.mockResolvedValueOnce({
			data: product('arugula', 'Руккола', { category: 'Овощи' }),
		});
		const { panel } = await openCarbonara();

		await searchProduct(panel, 'Руккола');
		await userEvent.click(
			await screen.findByRole('option', { name: 'Новый продукт «Руккола»' })
		);

		const dialog = await screen.findByRole('dialog', { name: 'Новый продукт' });

		expect(
			within(dialog).getByRole('textbox', { name: 'Название' })
		).toHaveValue('Руккола');

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Срок хранения, дней' }),
			'3'
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать продукт' })
		);

		expect(
			await within(panel).findByRole('textbox', {
				name: /^Количество: Руккола/,
			})
		).toHaveValue('');
		expect(post).toHaveBeenCalledWith(
			'/companies/demo/products',
			{ name: 'Руккола', unit: 'g', shelf_life_days: 3 },
			expect.anything()
		);
	});

	it('закрытие с изменённой рецептурой спрашивает подтверждение', async () => {
		const { panel, router } = await openCarbonara();

		await setQuantity(panel, 'Бекон', '70');
		await userEvent.click(
			within(panel).getByRole('button', { name: 'Закрыть блюдо' })
		);

		expect(
			await screen.findByRole('alertdialog', {
				name: 'Отбросить изменения рецептуры?',
			})
		).toBeVisible();
		expect(router.state.location.search).toEqual({ dish: 'carbonara' });
	});

	it('пустую рецептуру сохранить нельзя', async () => {
		const { panel } = await openCarbonara();

		await userEvent.click(
			within(panel).getByRole('button', { name: 'Убрать из рецептуры: Бекон' })
		);
		await userEvent.click(
			within(panel).getByRole('button', {
				name: 'Убрать из рецептуры: Сыр пармезан',
			})
		);

		expect(
			within(panel).getByText(/Рецептура не может быть пустой/)
		).toBeVisible();
		expect(
			within(panel).getByRole('button', { name: 'Сохранить рецептуру' })
		).toBeDisabled();
	});
});

describe('новое блюдо и архив блюд', () => {
	async function openNewDish() {
		await renderApp('/points/lenina/catalog/dishes');
		await userEvent.click(
			await screen.findByRole('button', { name: 'Новое блюдо' })
		);

		const dialog = await screen.findByRole('dialog', { name: 'Новое блюдо' });

		await userEvent.type(
			within(dialog).getByRole('textbox', { name: 'Название' }),
			'Салат капрезе'
		);

		return dialog;
	}

	it('блюдо создаётся с рецептурой из двух продуктов', async () => {
		post.mockResolvedValueOnce({ data: { ...soup, name: 'Салат капрезе' } });
		const dialog = await openNewDish();

		await searchProduct(dialog, 'моцарелла');
		await userEvent.click(
			await screen.findByRole('option', { name: /Сыр моцарелла/ })
		);
		await setQuantity(dialog, 'Сыр моцарелла', '120');
		await searchProduct(dialog, 'Бекон');
		await userEvent.click(await screen.findByRole('option', { name: /Бекон/ }));
		await setQuantity(dialog, 'Бекон', '40');
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать блюдо' })
		);

		await waitFor(() =>
			expect(post).toHaveBeenCalledWith(
				'/companies/demo/dishes',
				{
					name: 'Салат капрезе',
					ingredients: [
						{ product_id: 'mozzarella', quantity: 120 },
						{ product_id: 'bacon', quantity: 40 },
					],
				},
				expect.anything()
			)
		);
	});

	it('без продуктов рецептура не отправляется, диалог предупреждает о стоп-листе', async () => {
		post.mockResolvedValueOnce({ data: { ...soup, name: 'Салат капрезе' } });
		const dialog = await openNewDish();

		expect(
			within(dialog).getByText(/Без рецептуры блюдо сразу попадёт в стоп-лист/)
		).toBeVisible();

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Создать блюдо' })
		);

		await waitFor(() =>
			expect(post).toHaveBeenCalledWith(
				'/companies/demo/dishes',
				{ name: 'Салат капрезе' },
				expect.anything()
			)
		);
	});

	it('блюдо с архивным продуктом не восстанавливается, отказ называет продукт', async () => {
		await renderApp('/points/lenina/catalog/dishes?filter=archived');

		await userEvent.click(
			within(await dishesTable()).getByRole('button', {
				name: 'Восстановить: Карпаччо из лосося',
			})
		);

		const dialog = await screen.findByRole('dialog', {
			name: 'Нельзя восстановить блюдо',
		});

		expect(within(dialog).getByText('Лосось охлаждённый')).toBeVisible();
		expect(post).not.toHaveBeenCalled();
	});

	it('блюдо отправляется в архив после подтверждения', async () => {
		post.mockResolvedValueOnce({ data: { ...soup, archived: true } });
		await renderApp('/points/lenina/catalog/dishes');

		await userEvent.click(
			within(await dishesTable()).getByRole('button', {
				name: 'Действия: Суп томатный',
			})
		);
		await userEvent.click(
			await screen.findByRole('menuitem', { name: 'Отправить в архив' })
		);
		await userEvent.click(
			within(
				await screen.findByRole('alertdialog', {
					name: 'Отправить «Суп томатный» в архив?',
				})
			).getByRole('button', { name: 'В архив' })
		);

		expect(post).toHaveBeenCalledWith(
			'/companies/demo/dishes/soup/archive',
			undefined,
			expect.anything()
		);
	});
});
