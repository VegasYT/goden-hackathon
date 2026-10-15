import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	companyPath,
	getData,
	idempotencyHeader,
	type components,
} from '@shared/api';
import { catalogFreshMs, catalogKey, keepPointData } from '@shared/lib/query';

export type Product = components['schemas']['Product'];
export type NewProduct = components['schemas']['NewProduct'];
export type ProductChange = components['schemas']['ProductChange'];
type ProductPage = components['schemas']['ProductPage'];

/** Параметры списка продуктов: поиск, категорию, архив, сортировку и страницы считает сервер. */
export interface ProductListParams {
	q?: string;
	category?: string;
	status?: components['schemas']['CatalogStatus'];
	sort?: 'name' | 'category';
	limit: number;
	offset: number;
}

/** Ключи запросов продуктов под префиксом каталога компании. */
export const productKeys = {
	all: (companyId: string) => [...catalogKey(companyId), 'products'] as const,
	firstPage: (companyId: string) =>
		[...productKeys.all(companyId), 'first-page'] as const,
	list: (companyId: string, params: ProductListParams) =>
		[...productKeys.all(companyId), 'list', params] as const,
	detail: (companyId: string, productId: string) =>
		[...productKeys.all(companyId), 'detail', productId] as const,
	categories: (companyId: string) =>
		[...catalogKey(companyId), 'categories'] as const,
};

const productsPath = (companyId: string) =>
	`${companyPath(companyId)}/products`;

const productPath = (companyId: string, productId: string) =>
	`${productsPath(companyId)}/${encodeURIComponent(productId)}`;

/** Сколько продуктов каталога читается: одна страница API. */
const productLimit = 200;

/** Первая страница каталога компании по алфавиту; каталог меняется редко, поэтому не перечитывается несколько минут. */
export function catalogProductsQueryOptions(companyId: string) {
	return queryOptions({
		queryKey: productKeys.firstPage(companyId),
		queryFn: getData<ProductPage>(productsPath(companyId), {
			limit: productLimit,
		}),
		staleTime: catalogFreshMs,
	});
}

/** Страница продуктов каталога с фильтрами сервера. */
export function productListQueryOptions(
	companyId: string,
	params: ProductListParams
) {
	return queryOptions({
		queryKey: productKeys.list(companyId, params),
		queryFn: getData<ProductPage>(productsPath(companyId), params),
		placeholderData: keepPointData(companyId),
	});
}

/** Продукт каталога по идентификатору; только в этом ответе есть блюда с продуктом (`used_in`). */
export function productQueryOptions(companyId: string, productId: string) {
	return queryOptions({
		queryKey: productKeys.detail(companyId, productId),
		queryFn: getData<Product>(productPath(companyId, productId)),
		staleTime: catalogFreshMs,
	});
}

/** Категории каталога компании с числом действующих продуктов; продукты без категории в список не входят. */
export function productCategoriesQueryOptions(companyId: string) {
	return queryOptions({
		queryKey: productKeys.categories(companyId),
		queryFn: async (context) =>
			(
				await getData<components['schemas']['ProductCategories']>(
					`${companyPath(companyId)}/product-categories`
				)(context)
			).items.filter(({ name }) => name),
		staleTime: catalogFreshMs,
	});
}

export async function createProduct(
	companyId: string,
	product: NewProduct,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Product>(
			productsPath(companyId),
			product,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

export async function changeProduct(
	companyId: string,
	productId: string,
	change: ProductChange,
	idempotencyKey: string
) {
	return (
		await apiClient.patch<Product>(
			productPath(companyId, productId),
			change,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}

/** Отправляет продукт в архив или восстанавливает из архива. */
export async function setProductArchived(
	companyId: string,
	productId: string,
	archived: boolean,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Product>(
			`${productPath(companyId, productId)}/${archived ? 'archive' : 'restore'}`,
			undefined,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
