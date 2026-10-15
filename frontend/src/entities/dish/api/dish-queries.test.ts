import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';

import { apiClient } from '@shared/api';

import { pointDishesQueryOptions, type PointDish } from './dish-queries';

const dish = (index: number): PointDish => ({
	dish_id: `dish-${index}`,
	name: `Блюдо ${index}`,
	enabled: true,
});

describe('чтение блюд точки', () => {
	it('дочитывает следующие страницы, пока не наберёт всё', async () => {
		const get = vi
			.spyOn(apiClient, 'get')
			.mockResolvedValueOnce({
				data: {
					items: Array.from({ length: 200 }, (_, i) => dish(i)),
					total: 201,
				},
			})
			.mockResolvedValueOnce({ data: { items: [dish(200)], total: 201 } });

		const dishes = await new QueryClient().fetchQuery(
			pointDishesQueryOptions('lenina')
		);

		expect(dishes).toHaveLength(201);
		expect(get).toHaveBeenLastCalledWith('/points/lenina/dishes', {
			params: { limit: 200, offset: 200 },
			signal: expect.any(AbortSignal),
		});
	});
});
