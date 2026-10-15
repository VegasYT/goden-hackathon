import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	getData,
	idempotencyHeader,
	pointPath,
	type components,
} from '@shared/api';
import { pointKey } from '@shared/lib/query';

export type DishAvailability = components['schemas']['DishAvailability'];
export type DishRef = components['schemas']['DishRef'];
export type MenuIngredient = components['schemas']['LimitingIngredient'];
export type PointDish = components['schemas']['PointDish'];

/** Ключи запросов блюд точки: меню кухни и стоп-листа и блюда, выключенные на точке. */
export const dishKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'kitchen'] as const,
	menu: (pointId: string) => [...dishKeys.all(pointId), 'menu'] as const,
	dishes: (pointId: string) => [...dishKeys.all(pointId), 'dishes'] as const,
};

const pointDishesPath = (pointId: string) => `${pointPath(pointId)}/dishes`;

const pageLimit = 200;

interface Page<T> {
	items: T[];
	total: number;
}

// Меню и блюда точки нужны целиком: поиск по рецептуре и сортировка по порциям идут по всему списку.
async function readAllPages<T>(url: string, signal: AbortSignal) {
	const items: T[] = [];

	for (;;) {
		const data = await getData<Page<T>>(url, {
			limit: pageLimit,
			offset: items.length,
		})({ signal });

		items.push(...data.items);

		if (data.items.length === 0 || items.length >= data.total) {
			return items;
		}
	}
}

/** Все блюда, которые продаются на точке, с порциями и рецептурой по текущим остаткам. */
export function menuQueryOptions(pointId: string) {
	return queryOptions({
		queryKey: dishKeys.menu(pointId),
		queryFn: ({ signal }) =>
			readAllPages<DishAvailability>(`${pointPath(pointId)}/menu`, signal),
	});
}

/** Все блюда компании с признаком, продаются ли они на точке. */
export function pointDishesQueryOptions(pointId: string) {
	return queryOptions({
		queryKey: dishKeys.dishes(pointId),
		queryFn: ({ signal }) =>
			readAllPages<PointDish>(pointDishesPath(pointId), signal),
	});
}

/** Выключает или включает блюдо на точке. */
export async function setDishOnPoint(
	pointId: string,
	dishId: string,
	enabled: boolean,
	idempotencyKey: string
) {
	return (
		await apiClient.put<PointDish>(
			`${pointDishesPath(pointId)}/${encodeURIComponent(dishId)}`,
			{ enabled },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
