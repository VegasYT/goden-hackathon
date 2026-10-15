import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { ApiError } from '@shared/api';

import { lastParams, paramsOf, requestsTo } from './test-api';
import { renderApp } from './test-router';
import {
	expandTomato,
	openTomato,
	stockApi,
	testMovement,
	testStockSummary,
	tomatoLevel,
} from './test-stock';

const openStock = async (
	path = '/points/lenina/stock',
	options?: Parameters<typeof renderApp>[1]
) => {
	const view = await renderApp(path, options);

	await screen.findByRole('heading', { level: 1, name: 'Склад' });

	return view;
};

describe('остатки на ноутбуке', () => {
	it('статус и поиск уходят на сервер и в адрес, страница сбрасывается', async () => {
		const get = stockApi();
		const { router } = await openStock('/points/lenina/stock?page=2');

		await userEvent.click(
			await screen.findByRole('button', { name: /^Нет в наличии 1/ })
		);

		await waitFor(() =>
			expect(lastParams(get, '/stock')).toMatchObject({
				status: 'out_of_stock',
				offset: 0,
			})
		);
		expect(router.state.location.search).toEqual({ status: 'out_of_stock' });

		await userEvent.type(
			screen.getByRole('textbox', { name: 'Поиск по продукту' }),
			'том'
		);

		await waitFor(() =>
			expect(lastParams(get, '/stock')).toMatchObject({ q: 'том' })
		);
		expect(router.state.location.search).toMatchObject({ q: 'том' });
	});

	it('страница за концом списка открывает последнюю', async () => {
		stockApi({
			stock: (params) =>
				params.offset === 0
					? { items: [tomatoLevel], total: 1 }
					: { items: [], total: 1 },
		});
		const { router } = await openStock('/points/lenina/stock?page=9');

		expect(await screen.findByText('Томаты черри')).toBeInTheDocument();
		expect(router.state.location.search).toEqual({});
	});

	it('раскрытый продукт показывает партии в порядке расхода со ссылкой на поставку', async () => {
		stockApi();
		await openStock();

		expect(
			screen.queryByRole('button', { name: 'Партии продукта Моцарелла' })
		).toBeNull();
		expect(screen.getByText('нет в наличии')).toBeInTheDocument();

		await expandTomato();

		const batches = await screen.findByRole('table', {
			name: 'Партии продукта Томаты черри, в порядке расхода',
		});
		const [first] = within(batches).getAllByRole('row').slice(1);

		expect(within(first).getByText('расходуется первой')).toBeInTheDocument();
		expect(within(first).getByText('1,6 из 2 кг')).toBeInTheDocument();
		expect(
			within(first).getByRole('link', { name: 'Поставка № 111' })
		).toHaveAttribute('href', '/points/lenina/deliveries/d-111');
	});

	it('без партий на складе показывает пустой склад со ссылкой на поставки', async () => {
		stockApi({
			summary: testStockSummary({ in_stock: 0, expired: 0, out_of_stock: 2 }),
		});
		await openStock();

		expect(
			await screen.findByText('На складе пока нет партий')
		).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'К поставкам' })).toHaveAttribute(
			'href',
			'/points/lenina/deliveries'
		);
	});

	it('пустой результат поиска сбрасывается', async () => {
		stockApi({
			stock: (params) =>
				params.q ? { items: [], total: 0 } : { items: [tomatoLevel], total: 1 },
		});
		const { router } = await openStock('/points/lenina/stock?q=лосось');

		expect(await screen.findByText('Ничего не нашлось')).toBeInTheDocument();

		await userEvent.click(
			screen.getByRole('button', { name: 'Сбросить фильтры' })
		);

		expect(await screen.findByText('Томаты черри')).toBeInTheDocument();
		expect(router.state.location.search).toEqual({});
	});

	it('после сбоя загрузки «Повторить» запрашивает остатки заново', async () => {
		let online = false;

		stockApi({
			stock: () => {
				if (!online) {
					throw new ApiError('Нет связи с сервером');
				}

				return { items: [tomatoLevel], total: 1 };
			},
		});
		await openStock();

		const retry = await screen.findByRole('button', { name: 'Повторить' });

		online = true;
		await userEvent.click(retry);

		expect(await screen.findByText('Томаты черри')).toBeInTheDocument();
	});

	it('история открывается в панели, догружается и закрывается кнопкой «Назад»', async () => {
		const get = stockApi({
			movements: (offset) => ({
				items: [
					testMovement({
						id: `m-${offset}`,
						type: offset === 0 ? 'consumed' : 'received',
						delivery_id: 'd-111',
					}),
				],
				total: 2,
			}),
		});
		const { router } = await openTomato();

		await userEvent.click(
			await screen.findByRole('button', {
				name: 'История продукта Томаты черри',
			})
		);

		const panel = await screen.findByRole('dialog', {
			name: 'История · Томаты черри',
		});

		expect(router.state.location.search).toEqual({ history: 'tomato' });
		expect(
			await within(panel).findByText('Расход по заказу')
		).toBeInTheDocument();
		expect(within(panel).getByText('1 из 2')).toBeInTheDocument();

		await userEvent.click(
			within(panel).getByRole('button', { name: 'Показать ещё' })
		);

		expect(
			await within(panel).findByText('Получение партии')
		).toBeInTheDocument();
		expect(
			paramsOf(get, '/movements').some((params) => params.offset === 1)
		).toBe(true);

		act(() => router.history.back());

		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	});

	it('история запрашивает движения, не дожидаясь продукта', async () => {
		const get = stockApi();
		const answer = get.getMockImplementation()!;

		get.mockImplementation((url, config) =>
			url.endsWith('/products/tomato')
				? new Promise(() => {})
				: answer(url, config)
		);
		await renderApp('/points/lenina/stock?history=tomato');

		await screen.findByRole('dialog', { name: 'История продукта' });
		await waitFor(() => expect(requestsTo(get, '/movements')).toHaveLength(1));
	});
});

describe('остатки на планшете', () => {
	it('«Фильтры» меняют категорию и сортировку', async () => {
		const get = stockApi();
		const { router } = await openStock('/points/lenina/stock', { wide: false });

		await userEvent.click(
			await screen.findByRole('button', { name: 'Фильтры' })
		);

		const dialog = await screen.findByRole('dialog', { name: 'Фильтры' });

		await userEvent.click(
			within(dialog).getByRole('combobox', { name: 'Категория' })
		);
		// «Рыба» есть только в категориях каталога: у продуктов на складе её нет.
		expect(
			await screen.findByRole('option', { name: 'Рыба' })
		).toBeInTheDocument();
		await userEvent.click(
			await screen.findByRole('option', { name: 'Овощи и зелень' })
		);
		await userEvent.click(
			within(dialog).getByRole('combobox', { name: 'Сортировка' })
		);
		await userEvent.click(
			await screen.findByRole('option', { name: 'Сначала ближайший срок' })
		);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Показать' })
		);

		await waitFor(() =>
			expect(lastParams(get, '/stock')).toMatchObject({
				category: 'Овощи и зелень',
				sort: 'nearest_expiry',
			})
		);
		expect(router.state.location.search).toEqual({
			category: 'Овощи и зелень',
			sort: 'nearest_expiry',
		});
	});

	it('карточка продукта раскрывается в партии с историей', async () => {
		stockApi();
		await openStock('/points/lenina/stock', { wide: false });

		await userEvent.click(
			await screen.findByRole('button', {
				name: /Томаты черри/,
				expanded: false,
			})
		);

		const region = await screen.findByRole('region', {
			name: 'Партии «Томаты черри»',
		});

		expect(
			within(region).getByRole('button', {
				name: 'История продукта Томаты черри',
			})
		).toBeInTheDocument();
		expect(
			await within(region).findByRole('link', { name: 'Поставка № 113' })
		).toBeInTheDocument();
	});
});
