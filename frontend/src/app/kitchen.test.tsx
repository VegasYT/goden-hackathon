import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { apiClient, ApiError } from '@shared/api';

import { idempotencyKeyOf, requestsTo } from './test-api';
import { kitchenApi, pointDishes, testDish, testMenu } from './test-kitchen';
import {
	cook,
	manager,
	renderApp,
	testPoint,
	testProfile,
} from './test-router';

const dishRow = (name: string) =>
	screen.getByRole('row', { name: new RegExp(`^(Рецептура: )?${name}`) });

/** Блюда в порядке строк меню. */
const menuOrder = (names: string[]) => {
	const text = screen.getByRole('table', { name: 'Меню точки' }).textContent;

	return names
		.filter((name) => text.includes(name))
		.toSorted((a, b) => text.indexOf(a) - text.indexOf(b));
};

const allDishes = testMenu.map(({ dish }) => dish.name);

describe('кухня: меню', () => {
	it('показывает порции, стоп-лист с причиной и выключенные блюда', async () => {
		kitchenApi({ menu: testMenu });
		await renderApp('/points/lenina/kitchen');

		expect(
			await screen.findByText('5 блюд в меню, 3 в стоп-листе')
		).toBeInTheDocument();
		expect(dishRow('Паста карбонара')).toHaveTextContent(
			'15Сливки 33% · есть 1,2 л, на порцию 80 мл'
		);
		expect(dishRow('Брускетта с томатами')).toHaveTextContent(
			'стоп-листНе хватает: Томаты · есть 60 г, на порцию 100 г'
		);
		expect(dishRow('Куриное филе в сливочном соусе')).toHaveTextContent(
			'ещё 900 г просрочено'
		);
		expect(dishRow('Чизкейк Нью-Йорк')).toHaveTextContent(
			'Нет рецептуры: блюдо нельзя приготовить'
		);
		expect(
			screen.queryByRole('button', { name: 'Рецептура: Чизкейк Нью-Йорк' })
		).not.toBeInTheDocument();
		expect(screen.getByText('Выключены на точке · 1')).toBeInTheDocument();
		expect(dishRow('Солянка сборная')).toHaveTextContent(
			'не продаётся на точке'
		);
	});

	it('рецептура показывает остаток каждого продукта и на сколько порций его хватит', async () => {
		kitchenApi({ menu: testMenu });
		await renderApp('/points/lenina/kitchen');

		await userEvent.click(
			await screen.findByRole('button', { name: 'Рецептура: Паста карбонара' })
		);

		const recipe = screen.getByRole('table', {
			name: 'Рецептура блюда Паста карбонара на одну порцию',
		});
		const row = (product: string) =>
			within(recipe).getByRole('row', { name: new RegExp(`^${product}`) });

		expect(row('Бекон')).toHaveTextContent(
			'Бекон60 г1,3 кгещё 200 г просрочено21 порцию'
		);
		expect(row('Сливки 33%')).toHaveTextContent(
			'Сливки 33%ограничивает80 мл1,2 л15 порций'
		);
		expect(
			screen.getByText(/Заказ на 15 порций будет принят, на 16 — нет/)
		).toBeInTheDocument();
	});

	it('поиск по продукту и сортировка по порциям сохраняются в адресе', async () => {
		kitchenApi({ menu: testMenu });
		const { router } = await renderApp('/points/lenina/kitchen');

		await userEvent.type(
			await screen.findByRole('textbox', {
				name: 'Поиск по блюду или продукту',
			}),
			'бекон'
		);

		await waitFor(() =>
			expect(router.state.location.search).toEqual({ q: 'бекон' })
		);
		expect(menuOrder(allDishes)).toEqual(['Паста карбонара']);

		await userEvent.click(
			screen.getByRole('button', { name: 'Очистить поиск' })
		);
		await userEvent.click(
			screen.getByRole('combobox', { name: 'Сортировка меню' })
		);
		await userEvent.click(
			screen.getByRole('option', { name: 'Меньше порций сначала' })
		);

		await waitFor(() =>
			expect(router.state.location.search).toEqual({ sort: 'portions' })
		);
		expect(menuOrder(allDishes)).toEqual([
			'Брускетта с томатами',
			'Куриное филе в сливочном соусе',
			'Чизкейк Нью-Йорк',
			'Омлет',
			'Паста карбонара',
		]);
	});

	it('поиск без результатов сбрасывается', async () => {
		kitchenApi({ menu: testMenu });
		await renderApp('/points/lenina/kitchen?q=лосось', { wide: false });

		expect(
			await screen.findByText('Ничего не нашлось по «лосось»')
		).toBeInTheDocument();

		await userEvent.click(
			screen.getByRole('button', { name: 'Сбросить поиск' })
		);

		expect(
			await screen.findByRole('listitem', { name: 'Паста карбонара' })
		).toBeInTheDocument();
	});

	it('ошибка загрузки меню показывает текст и повтор', async () => {
		let fail = true;
		const api = kitchenApi({ menu: testMenu });
		const respond = api.getMockImplementation();

		api.mockImplementation(async (url, config) => {
			if (fail && url.endsWith('/menu')) {
				throw new ApiError('Нет связи с сервером', { status: 0 });
			}

			return respond!(url, config);
		});
		await renderApp('/points/lenina/kitchen', { wide: false });

		expect(await screen.findByText('Нет связи с сервером')).toBeInTheDocument();

		fail = false;
		await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));

		expect(
			await screen.findByRole('listitem', { name: 'Паста карбонара' })
		).toBeInTheDocument();
	});
});

const put = vi.spyOn(apiClient, 'put');

/** Меню, в котором сервер выключает и включает блюда по `PUT`. */
function switchableKitchen() {
	const state = { menu: testMenu, dishes: pointDishes(testMenu) };
	const kitchen = kitchenApi(state);

	put.mockImplementation(async (url, body) => {
		const { enabled } = body as { enabled: boolean };
		const dishId = decodeURIComponent(url.split('/').at(-1)!);

		state.dishes = state.dishes.map((dish) =>
			dish.dish_id === dishId ? { ...dish, enabled } : dish
		);
		state.menu = enabled
			? [...state.menu, testDish('Солянка сборная', [])]
			: state.menu.filter(({ dish }) => dish.id !== dishId);

		return { data: state.dishes.find((dish) => dish.dish_id === dishId) };
	});

	return kitchen;
}

describe('кухня: выключение и включение блюда', () => {
	it('выключает блюдо после подтверждения, «Отмена» ничего не отправляет', async () => {
		const kitchen = switchableKitchen();

		await renderApp('/points/lenina/kitchen');

		await userEvent.click(
			await screen.findByRole('button', {
				name: 'Выключить на точке: Брускетта с томатами',
			})
		);
		await userEvent.click(screen.getByRole('button', { name: 'Отмена' }));
		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		expect(put).not.toHaveBeenCalled();

		await userEvent.click(
			screen.getByRole('button', {
				name: 'Выключить на точке: Брускетта с томатами',
			})
		);
		const dialog = screen.getByRole('alertdialog', {
			name: 'Выключить «Брускетта с томатами»?',
		});
		const menuBefore = requestsTo(kitchen, '/menu').length;

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Выключить' })
		);

		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		expect(put).toHaveBeenCalledWith(
			`/points/lenina/dishes/${encodeURIComponent('dish-Брускетта с томатами')}`,
			{ enabled: false },
			{ headers: { 'Idempotency-Key': expect.any(String) } }
		);
		expect(screen.getByText('Выключены на точке · 2')).toBeInTheDocument();
		expect(dishRow('Брускетта с томатами')).toHaveTextContent(
			'не продаётся на точке'
		);
		expect(
			screen.getByRole('button', {
				name: 'Включить на точке: Брускетта с томатами',
			})
		).toBeInTheDocument();
		expect(requestsTo(kitchen, '/menu').length).toBe(menuBefore);
	});

	it('после обрыва повторяет выключение с тем же ключом, новое открытие — с новым', async () => {
		switchableKitchen();
		const succeed = put.getMockImplementation()!;

		put.mockRejectedValueOnce(
			new ApiError('Нет связи с сервером', { status: 0 })
		);
		await renderApp('/points/lenina/kitchen');

		const open = async () => {
			await userEvent.click(
				await screen.findByRole('button', { name: 'Выключить на точке: Омлет' })
			);
			return screen.getByRole('alertdialog', { name: 'Выключить «Омлет»?' });
		};

		let dialog = await open();

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Выключить' })
		);
		expect(
			await within(dialog).findByText('Нет связи с сервером')
		).toBeInTheDocument();

		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Отмена' })
		);
		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);

		put.mockRejectedValueOnce(
			new ApiError('Нет связи с сервером', { status: 0 })
		);
		dialog = await open();
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Выключить' })
		);
		await within(dialog).findByText('Нет связи с сервером');

		put.mockImplementation(succeed);
		await userEvent.click(
			within(dialog).getByRole('button', { name: 'Выключить' })
		);
		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);

		const [first, second, retry] = put.mock.calls.map(idempotencyKeyOf);

		expect(second).not.toBe(first);
		expect(retry).toBe(second);
	});

	it('включает блюдо без подтверждения, ошибку показывает над меню', async () => {
		switchableKitchen();
		const succeed = put.getMockImplementation()!;

		put.mockRejectedValueOnce(
			new ApiError('Нет связи с сервером', { status: 0 })
		);
		await renderApp('/points/lenina/kitchen');

		const enable = await screen.findByRole('button', {
			name: 'Включить на точке: Солянка сборная',
		});

		await userEvent.click(enable);
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Не удалось включить «Солянка сборная»: Нет связи с сервером'
		);

		put.mockImplementation(succeed);
		await userEvent.click(enable);

		await waitFor(() =>
			expect(screen.queryByText(/Выключены на точке/)).not.toBeInTheDocument()
		);
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
		expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
		expect(put).toHaveBeenLastCalledWith(
			`/points/lenina/dishes/${encodeURIComponent('dish-Солянка сборная')}`,
			{ enabled: true },
			expect.anything()
		);
	});

	it('без права управлять меню и на отключённой точке действий нет, над меню пояснение', async () => {
		kitchenApi({ menu: testMenu });
		let view = await renderApp('/points/lenina/kitchen', {
			profile: testProfile([manager]),
		});

		expect(
			await screen.findByRole('button', { name: 'Рецептура: Омлет' })
		).toBeInTheDocument();
		expect(screen.queryByRole('note')).toBeNull();
		view.unmount();

		view = await renderApp('/points/lenina/kitchen', {
			profile: testProfile([cook]),
		});

		expect(
			await screen.findByRole('button', { name: 'Рецептура: Омлет' })
		).toBeInTheDocument();
		expect(screen.getByRole('note')).toHaveTextContent(
			'Выключать и включать блюда могут управляющий и владелец'
		);
		expect(
			screen.queryByRole('button', { name: /^Выключить на точке/ })
		).toBeNull();
		expect(
			screen.queryByRole('button', { name: /^Включить на точке/ })
		).toBeNull();
		view.unmount();

		await renderApp('/points/lenina/kitchen', {
			profile: testProfile([testPoint({ active: false })]),
		});

		expect(
			await screen.findByRole('button', { name: 'Рецептура: Омлет' })
		).toBeInTheDocument();
		expect(screen.getByRole('note')).toHaveTextContent(
			'Точка отключена — выключать и включать блюда нельзя'
		);
		expect(
			screen.queryByRole('button', { name: /^Выключить на точке/ })
		).toBeNull();
		expect(
			screen.queryByRole('button', { name: /^Включить на точке/ })
		).toBeNull();
	});
});
