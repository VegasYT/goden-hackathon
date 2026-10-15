import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Product } from '@entities/product';
import { apiClient, ApiError, type components } from '@shared/api';
import { retype } from '@shared/lib/test-utils';

import { idempotencyKeyOf, mockGet } from './test-api';
import { testDelivery } from './test-deliveries';
import { testProduct } from './test-stock';
import {
	manager,
	navSections,
	renderApp,
	testPoint,
	testProfile,
} from './test-router';

type GeneratorSettings = components['schemas']['GeneratorSettings'];

const stopped: GeneratorSettings = {
	running: false,
	deliveries_per_minute: 6,
	orders_per_minute: 30,
};
const running: GeneratorSettings = {
	running: true,
	deliveries_per_minute: 6,
	orders_per_minute: 30,
	updated_at: '2026-10-13T11:20:00Z',
};

const catalog = ['Пармезан', 'Лосось', 'Сливки', 'Шпинат'].map((name, index) =>
	testProduct({ id: `product-${index}`, name })
);

const managerProfile = testProfile([manager]);

const put = vi.spyOn(apiClient, 'put');
const post = vi.spyOn(apiClient, 'post');

function generatorApi({
	settings = stopped,
	products = catalog,
	waiting = 0,
}: {
	settings?: GeneratorSettings;
	products?: Product[];
	waiting?: number;
} = {}) {
	return mockGet((url) => {
		if (url.endsWith('/generator')) {
			return settings;
		}

		if (url.endsWith('/products')) {
			return { items: products, total: products.length };
		}

		if (url.endsWith('/deliveries')) {
			return { items: [], total: waiting };
		}

		throw new ApiError('Не найдено', { status: 404 });
	});
}

async function openGenerator(options?: Parameters<typeof generatorApi>[0]) {
	const get = generatorApi(options);
	const view = await renderApp('/points/lenina/generator');

	await screen.findByRole('heading', { level: 1, name: 'Генератор' });

	return { ...view, get };
}

const generatorCard = () =>
	screen.getByRole('region', { name: 'Генератор событий' });
const testCard = () =>
	screen.getByRole('region', { name: 'Тестовая поставка' });

const setRate = (label: string, value: string) =>
	retype(within(generatorCard()).getByLabelText(label), value);

beforeEach(() => {
	put.mockImplementation(async (_url, body) => ({
		data: { ...(body as object), updated_at: '2026-10-13T11:25:00Z' },
	}));
});

describe('раздел «Генератор»', () => {
	it('владелец видит «Генератор» последним пунктом навигации и открывает раздел', async () => {
		await openGenerator();

		expect(navSections().at(-1)).toBe('Генератор');
		expect(generatorCard()).toHaveTextContent('остановлен');
		expect(generatorCard()).toHaveTextContent(
			'на этой точке ещё не запускался'
		);
	});

	it('управляющему пункт не виден, а адрес раздела показывает «Нет доступа»', async () => {
		generatorApi();
		await renderApp('/points/lenina/generator', { profile: managerProfile });

		expect(
			await screen.findByRole('heading', { name: 'Нет доступа к разделу' })
		).toBeInTheDocument();
		expect(navSections()).not.toContain('Генератор');
	});

	it('выключенная настройка сборки убирает пункт и страницу', async () => {
		vi.stubEnv('VITE_GENERATOR_ENABLED', 'false');
		generatorApi();
		const { router } = await renderApp('/points/lenina');

		expect(navSections()).not.toContain('Генератор');

		await act(() =>
			router.navigate({
				to: '/points/$pointId/generator',
				params: { pointId: 'lenina' },
			})
		);

		expect(
			await screen.findByRole('heading', { name: 'Страница не найдена' })
		).toBeInTheDocument();
	});
});

describe('генератор событий', () => {
	it('«Запустить» сохраняет введённую интенсивность', async () => {
		await openGenerator();

		await setRate('Поставок в минуту', '2');
		await setRate('Заказов в минуту', '0');
		await userEvent.click(
			within(generatorCard()).getByRole('button', { name: 'Запустить' })
		);

		await waitFor(() => expect(put).toHaveBeenCalledTimes(1));
		expect(put.mock.calls[0][0]).toBe('/points/lenina/generator');
		expect(put.mock.calls[0][1]).toEqual({
			running: true,
			deliveries_per_minute: 2,
			orders_per_minute: 0,
		});
		expect(await within(generatorCard()).findByText('работает')).toBeVisible();
	});

	it('«Остановить» не отправляет несохранённые правки интенсивности', async () => {
		await openGenerator({ settings: running });

		await setRate('Заказов в минуту', '10');
		await userEvent.click(
			within(generatorCard()).getByRole('button', { name: 'Остановить' })
		);

		await waitFor(() => expect(put).toHaveBeenCalledTimes(1));
		expect(put.mock.calls[0][1]).toEqual({
			running: false,
			deliveries_per_minute: 6,
			orders_per_minute: 30,
		});
	});

	it('значение больше 600 показывает ошибку и не отправляется', async () => {
		await openGenerator({ settings: running });

		await setRate('Поставок в минуту', '1000');
		await userEvent.click(
			within(generatorCard()).getByRole('button', { name: 'Сохранить' })
		);

		expect(within(generatorCard()).getByText('Не больше 600')).toBeVisible();
		expect(put).not.toHaveBeenCalled();
	});

	it('ошибка запуска показывается текстом, генератор остаётся остановленным', async () => {
		put.mockRejectedValueOnce(new ApiError('Точка отключена', { status: 409 }));
		await openGenerator();

		await userEvent.click(
			within(generatorCard()).getByRole('button', { name: 'Запустить' })
		);

		expect(
			await within(generatorCard()).findByText('Точка отключена')
		).toBeVisible();
		expect(generatorCard()).toHaveTextContent('остановлен');
	});

	it('на отключённой точке включённый генератор можно только остановить', async () => {
		generatorApi({ settings: running });
		await renderApp('/points/lenina/generator', {
			profile: testProfile([testPoint({ active: false })]),
		});
		await screen.findByRole('heading', { level: 1, name: 'Генератор' });

		const card = generatorCard();

		expect(
			within(card).getByRole('button', { name: 'Остановить' })
		).toBeEnabled();
		expect(
			within(card).queryByRole('button', { name: 'Запустить' })
		).toBeNull();
		expect(
			within(card).queryByRole('button', { name: 'Сохранить' })
		).toBeNull();
		expect(
			within(testCard()).getByRole('button', { name: 'Привезти поставку' })
		).toBeDisabled();
	});
});

describe('тестовая поставка', () => {
	it('регистрирует 3 позиции и просроченную с ключом и ведёт в карточку из сообщения', async () => {
		const brought = testDelivery({
			id: 'delivery-test',
			number: 'ТЕСТ-1013-143205',
		});

		post.mockResolvedValueOnce({ data: brought });
		const { router } = await openGenerator();

		await userEvent.click(
			within(testCard()).getByRole('checkbox', { name: /Просроченную/ })
		);
		await userEvent.click(
			within(testCard()).getByRole('button', { name: 'Привезти поставку' })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(1));
		const [url, body] = post.mock.calls[0] as [
			string,
			{ number: string; items: { expires_at: string }[] },
		];

		expect(url).toBe('/points/lenina/deliveries');
		expect(body.number).toMatch(/^ТЕСТ-/);
		expect(body.items).toHaveLength(4);
		expect(
			body.items.filter((item) => new Date(item.expires_at) < new Date())
		).toHaveLength(1);
		expect(idempotencyKeyOf(post.mock.calls[0])).toEqual(expect.any(String));

		expect(
			await screen.findByText(
				'Поставка № ТЕСТ-1013-143205 зарегистрирована · 1 позиция'
			)
		).toBeVisible();

		await userEvent.click(screen.getByRole('link', { name: 'Открыть' }));
		await waitFor(() =>
			expect(router.state.location.pathname).toBe(
				'/points/lenina/deliveries/delivery-test'
			)
		);
	});

	it('после сбоя «Повторить» отправляет ту же поставку с тем же ключом', async () => {
		post
			.mockRejectedValueOnce(new ApiError('Нет связи с сервером'))
			.mockResolvedValueOnce({ data: testDelivery({ number: 'ТЕСТ-1' }) });
		await openGenerator();

		await userEvent.click(
			within(testCard()).getByRole('button', { name: 'Привезти поставку' })
		);
		await userEvent.click(
			await within(testCard()).findByRole('button', { name: 'Повторить' })
		);

		await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
		expect(post.mock.calls[1][1]).toEqual(post.mock.calls[0][1]);
		expect(idempotencyKeyOf(post.mock.calls[1])).toBe(
			idempotencyKeyOf(post.mock.calls[0])
		);
	});

	it('при пустом каталоге «Привезти поставку» неактивна и есть пояснение', async () => {
		await openGenerator({ products: [] });

		expect(
			await within(testCard()).findByText(/В каталоге компании нет продуктов/)
		).toBeVisible();
		expect(
			within(testCard()).getByRole('button', { name: 'Привезти поставку' })
		).toBeDisabled();
	});

	it('показывает, сколько тестовых поставок ждут приёмки', async () => {
		const { get } = await openGenerator({ waiting: 2 });

		expect(
			await within(testCard()).findByText('Ждут приёмки 2 тестовые поставки')
		).toBeVisible();
		expect(get).toHaveBeenCalledWith(
			'/points/lenina/deliveries',
			expect.objectContaining({
				params: expect.objectContaining({
					status: 'registered',
					number: 'ТЕСТ-',
				}),
			})
		);
	});
});
