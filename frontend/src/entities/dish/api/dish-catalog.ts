import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	companyPath,
	getData,
	idempotencyHeader,
	type components,
} from '@shared/api';
import { catalogKey, keepPointData } from '@shared/lib/query';

export type Dish = components['schemas']['Dish'];
export type Ingredient = components['schemas']['Ingredient'];
type DishPage = components['schemas']['DishPage'];

/** Параметры списка блюд компании: поиск, архив и блюда без рецептуры считает сервер. */
export interface DishListParams {
	q?: string;
	status?: components['schemas']['CatalogStatus'];
	recipe?: 'missing';
	limit: number;
	offset: number;
}

/** Ключи запросов блюд каталога под префиксом каталога компании. */
export const dishCatalogKeys = {
	all: (companyId: string) => [...catalogKey(companyId), 'dishes'] as const,
	list: (companyId: string, params: DishListParams) =>
		[...dishCatalogKeys.all(companyId), 'list', params] as const,
	detail: (companyId: string, dishId: string) =>
		[...dishCatalogKeys.all(companyId), 'detail', dishId] as const,
};

const dishesPath = (companyId: string) => `${companyPath(companyId)}/dishes`;

const dishPath = (companyId: string, dishId: string) =>
	`${dishesPath(companyId)}/${encodeURIComponent(dishId)}`;

/** Страница блюд каталога компании с рецептурами. */
export function companyDishesQueryOptions(
	companyId: string,
	params: DishListParams
) {
	return queryOptions({
		queryKey: dishCatalogKeys.list(companyId, params),
		queryFn: getData<DishPage>(dishesPath(companyId), params),
		placeholderData: keepPointData(companyId),
	});
}

/** Блюдо каталога с рецептурой. */
export function dishQueryOptions(companyId: string, dishId: string) {
	return queryOptions({
		queryKey: dishCatalogKeys.detail(companyId, dishId),
		queryFn: getData<Dish>(dishPath(companyId, dishId)),
	});
}

/** Создаёт блюдо; пустую рецептуру не отправляет, потому что API ждёт хотя бы один продукт. */
export async function createDish(
	companyId: string,
	dish: { name: string; ingredients: Ingredient[] },
	idempotencyKey: string
) {
	const body = dish.ingredients.length > 0 ? dish : { name: dish.name };

	return (
		await apiClient.post<Dish>(
			dishesPath(companyId),
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function renameDish(
	companyId: string,
	dishId: string,
	name: string,
	idempotencyKey: string
) {
	return (
		await apiClient.patch<Dish>(
			dishPath(companyId, dishId),
			{ name },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

/** Заменяет рецептуру блюда целиком. */
export async function changeRecipe(
	companyId: string,
	dishId: string,
	ingredients: Ingredient[],
	idempotencyKey: string
) {
	return (
		await apiClient.put<Dish>(
			`${dishPath(companyId, dishId)}/recipe`,
			{ ingredients },
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

/** Отправляет блюдо в архив или восстанавливает из архива. */
export async function setDishArchived(
	companyId: string,
	dishId: string,
	archived: boolean,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Dish>(
			`${dishPath(companyId, dishId)}/${archived ? 'archive' : 'restore'}`,
			undefined,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
