import * as z from 'zod/mini';

import type { DishListParams } from '@entities/dish';
import type { ProductListParams } from '@entities/product';
import {
	pageFields,
	textField,
	toPageParams,
	withDefault,
} from '@shared/lib/search-params';

/** Состояние вкладки «Продукты» в адресе. */
export const productsSearchSchema = z.object({
	q: textField(),
	category: textField(),
	archived: withDefault(z.boolean(), false),
	sort: withDefault(z.enum(['name', 'category']), 'name'),
	...pageFields,
});

export type ProductsSearch = z.output<typeof productsSearchSchema>;

/** Значения по умолчанию: их нет в адресе. */
export const productsSearchDefaults = productsSearchSchema.parse({});

export function toProductParams(search: ProductsSearch): ProductListParams {
	return {
		q: search.q.trim() || undefined,
		category: search.category || undefined,
		status: search.archived ? 'archived' : undefined,
		sort: search.sort === 'category' ? 'category' : undefined,
		...toPageParams(search),
	};
}

/** Поиск или категория сужают список продуктов. */
export function isProductsFiltered(search: ProductsSearch): boolean {
	return search.q.trim() !== '' || search.category !== '';
}

export const dishFilters = ['active', 'missing', 'archived'] as const;

export type DishFilter = (typeof dishFilters)[number];

/** Состояние вкладки «Блюда» в адресе; `dish` — блюдо, открытое в панели. */
export const dishesSearchSchema = z.object({
	q: textField(),
	filter: withDefault(z.enum(dishFilters), 'active'),
	...pageFields,
	dish: z.catch(z.optional(z.string()), undefined),
});

export type DishesSearch = z.output<typeof dishesSearchSchema>;

export const dishesSearchDefaults = dishesSearchSchema.parse({});

/** Параметры запроса блюд по фильтру; без страницы — для счётчиков. */
export function dishFilterParams(
	filter: DishFilter
): Pick<DishListParams, 'status' | 'recipe'> {
	if (filter === 'archived') {
		return { status: 'archived' };
	}

	return filter === 'missing' ? { recipe: 'missing' } : {};
}

export function toDishParams(search: DishesSearch): DishListParams {
	return {
		q: search.q.trim() || undefined,
		...dishFilterParams(search.filter),
		...toPageParams(search),
	};
}
