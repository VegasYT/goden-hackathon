import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { rowCells } from '@shared/lib/test-utils';

import { requestsTo } from './test-api';
import {
	bruschetta,
	carbonara,
	cheesecake,
	chicken,
	kitchenApi,
	omelet,
	testDish,
	testIngredient,
	testMenu,
} from './test-kitchen';
import { cook, manager, renderApp, testProfile } from './test-router';

const omeletShort = testDish('Омлет', [
	testIngredient('Яйцо куриное', 'pcs', 3, 2),
	testIngredient('Молоко', 'ml', 50, 0, 1500),
]);

describe('стоп-лист', () => {
	it('объясняет причины: нехватку двух продуктов, просрочку и нет рецептуры', async () => {
		kitchenApi({
			menu: [bruschetta, chicken, omeletShort, cheesecake, carbonara],
		});
		await renderApp('/points/lenina/stop-list');

		expect(
			await screen.findByText(/^4 блюда нельзя приготовить · на \d\d:\d\d$/)
		).toBeInTheDocument();
		expect(
			screen.queryByRole('article', { name: 'Паста карбонара' })
		).not.toBeInTheDocument();

		const shortage = (dish: string, product: string) =>
			within(
				screen.getByRole('table', {
					name: `Чего не хватает на порцию: ${dish}`,
				})
			).getByRole('row', { name: new RegExp(`^${product}`) });

		expect(rowCells(shortage('Брускетта с томатами', 'Томаты'))).toEqual([
			'Томаты',
			'100 г',
			'60 г',
			'40 г',
			'—',
		]);
		expect(rowCells(shortage('Омлет', 'Яйцо куриное'))).toEqual([
			'Яйцо куриное',
			'3 шт',
			'2 шт',
			'1 шт',
			'—',
		]);
		expect(rowCells(shortage('Омлет', 'Молоко'))).toEqual([
			'Молоко',
			'50 мл',
			'0 мл',
			'50 мл',
			expect.stringMatching(/^1,5 л/),
		]);

		const chickenFillet = shortage(
			'Куриное филе в сливочном соусе',
			'Куриное филе'
		);

		expect(within(chickenFillet).getByText('900 г')).toBeInTheDocument();
		expect(
			within(chickenFillet).getByText(
				'в просроченных партиях, готовить из них нельзя'
			)
		).toBeInTheDocument();

		const noRecipe = screen.getByRole('article', { name: 'Чизкейк Нью-Йорк' });

		expect(noRecipe).toHaveTextContent('нет рецептуры');
		expect(noRecipe).toHaveTextContent('У блюда не задана рецептура');
		expect(
			within(noRecipe).getByRole('link', { name: '«Кухня»' })
		).toHaveAttribute('href', '/points/lenina/kitchen');
	});

	it('управляющему даёт ссылку на склад точки и на историю недостающего продукта', async () => {
		kitchenApi({ menu: [omeletShort] });
		await renderApp('/points/lenina/stop-list', {
			profile: testProfile([manager]),
		});

		expect(
			await screen.findByRole('link', { name: 'Открыть склад' })
		).toHaveAttribute('href', '/points/lenina/stock');
		expect(screen.getByRole('link', { name: 'Молоко' })).toHaveAttribute(
			'href',
			`/points/lenina/stock?history=${encodeURIComponent('product-Молоко')}`
		);
	});

	it('повар не видит ссылок на склад, и данные склада не запрашиваются', async () => {
		const api = kitchenApi({ menu: testMenu });
		const profile = testProfile([cook]);
		const view = await renderApp('/points/lenina/kitchen', { profile });

		await screen.findByRole('button', { name: 'Рецептура: Омлет' });
		expect(screen.queryByRole('link', { name: /склад/i })).toBeNull();
		view.unmount();

		await renderApp('/points/lenina/stop-list', { profile });

		const noRecipe = await screen.findByRole('article', {
			name: 'Чизкейк Нью-Йорк',
		});

		expect(screen.queryByRole('link', { name: /склад/i })).toBeNull();
		expect(within(noRecipe).queryByRole('link')).toBeNull();
		expect(screen.getAllByText('Томаты').length).toBeGreaterThan(0);
		expect(screen.queryByRole('link', { name: 'Томаты' })).toBeNull();
		expect(requestsTo(api, /\/(stock|batches)/)).toEqual([]);
	});

	it('без блюд в стоп-листе говорит, что всё можно приготовить', async () => {
		kitchenApi({ menu: [carbonara, omelet] });
		await renderApp('/points/lenina/stop-list');

		expect(
			await screen.findByText('Все блюда можно приготовить')
		).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Открыть кухню' })).toHaveAttribute(
			'href',
			'/points/lenina/kitchen'
		);
	});
});

describe('стоп-лист на планшете', () => {
	it('пояснение о просрочке стоит под числом и не повторяет количество', async () => {
		kitchenApi({ menu: [chicken] });
		await renderApp('/points/lenina/stop-list', { wide: false });

		const card = await screen.findByRole('article', {
			name: 'Куриное филе в сливочном соусе',
		});

		expect(
			within(card).getAllByText(
				'в просроченных партиях, готовить из них нельзя'
			)
		).toHaveLength(1);
		expect(within(card).getAllByText('900 г')).toHaveLength(1);
	});
});
