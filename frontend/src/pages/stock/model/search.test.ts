import { describe, expect, it } from 'vitest';

import { stockSearchSchema, toFilterParams, toStockParams } from './search';

describe('адрес склада', () => {
	it('неверные значения заменяются умолчаниями', () => {
		expect(
			stockSearchSchema.parse({
				status: 'lost',
				sort: 'price',
				page: 0,
				size: 500,
			})
		).toEqual({
			q: '',
			category: '',
			status: 'all',
			sort: 'name',
			page: 1,
			size: 10,
		});
	});

	it('фильтры и страница переходят в параметры запроса', () => {
		const search = stockSearchSchema.parse({
			q: ' мол ',
			category: 'Молочные продукты',
			status: 'expired',
			sort: '-nearest_expiry',
			page: 3,
			size: 25,
		});

		expect(toStockParams(search)).toEqual({
			q: 'мол',
			category: 'Молочные продукты',
			status: 'expired',
			sort: '-nearest_expiry',
			limit: 25,
			offset: 50,
		});
		expect(toFilterParams(search)).toEqual({
			q: 'мол',
			category: 'Молочные продукты',
		});
	});

	it('статус «Все» и пустые фильтры не отправляются', () => {
		expect(toStockParams(stockSearchSchema.parse({}))).toEqual({
			q: undefined,
			category: undefined,
			status: undefined,
			sort: 'name',
			limit: 10,
			offset: 0,
		});
	});
});
