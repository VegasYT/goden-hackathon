import {
	hashKey,
	useMutation,
	useQueryClient,
	type QueryClient,
	type QueryKey,
} from '@tanstack/react-query';

import {
	changeRecipe,
	createDish,
	dishCatalogKeys,
	dishKeys,
	renameDish,
	setDishArchived,
	type Dish,
	type Ingredient,
} from '@entities/dish';
import {
	changeProduct,
	createProduct,
	productKeys,
	setProductArchived,
	type NewProduct,
	type Product,
	type ProductChange,
} from '@entities/product';
import { stockKeys } from '@entities/stock';
import { useIdempotencyKey } from '@shared/lib/idempotency';
import { catalogKey, invalidateKeys } from '@shared/lib/query';

import { useCatalogScope, type CatalogScope } from '../model/catalog-scope';

/**
 * Помечает устаревшими каталог компании, склад и меню точки: там видны названия, единицы и рецептуры.
 * `fresh` — ключ, данные которого только что пришли ответом записи; он не перечитывается.
 */
export function refreshCatalog(
	queryClient: QueryClient,
	companyId: string,
	pointId: string,
	fresh?: QueryKey
) {
	const freshHash = fresh && hashKey(fresh);

	invalidateKeys(
		queryClient,
		[catalogKey(companyId), stockKeys.all(pointId), dishKeys.all(pointId)],
		{ predicate: (query) => query.queryHash !== freshHash }
	);
}

/** Запись каталога: ответ кладётся в кэш продукта или блюда, остальной каталог перечитывается. */
function useCatalogWrite<Variables, Result>(
	write: (
		scope: CatalogScope,
		variables: Variables,
		idempotencyKey: string
	) => Promise<Result>,
	detailKey: (companyId: string, result: Result) => QueryKey
) {
	const scope = useCatalogScope();
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: (variables: Variables) =>
			write(scope, variables, keyFor(variables)),
		onSuccess: (result) => {
			const key = detailKey(scope.companyId, result);

			queryClient.setQueryData(key, result);
			refreshCatalog(queryClient, scope.companyId, scope.pointId, key);
		},
	});
}

const productDetail = (companyId: string, product: Product) =>
	productKeys.detail(companyId, product.id);

const dishDetail = (companyId: string, dish: Dish) =>
	dishCatalogKeys.detail(companyId, dish.id);

/** Создаёт продукт или, с `productId`, меняет его. */
export function useSaveProduct(productId?: string) {
	return useCatalogWrite(
		({ companyId }, body: NewProduct | ProductChange, key) =>
			productId
				? changeProduct(companyId, productId, body, key)
				: createProduct(companyId, body as NewProduct, key),
		productDetail
	);
}

/** Архив или восстановление продукта. */
export function useProductArchived() {
	return useCatalogWrite(
		({ companyId }, { id, archived }: { id: string; archived: boolean }, key) =>
			setProductArchived(companyId, id, archived, key),
		productDetail
	);
}

export function useCreateDish() {
	return useCatalogWrite(
		({ companyId }, dish: { name: string; ingredients: Ingredient[] }, key) =>
			createDish(companyId, dish, key),
		dishDetail
	);
}

export function useRenameDish(dishId: string) {
	return useCatalogWrite(
		({ companyId }, name: string, key) =>
			renameDish(companyId, dishId, name, key),
		dishDetail
	);
}

export function useChangeRecipe(dishId: string) {
	return useCatalogWrite(
		({ companyId }, ingredients: Ingredient[], key) =>
			changeRecipe(companyId, dishId, ingredients, key),
		dishDetail
	);
}

/** Архив или восстановление блюда. */
export function useDishArchived() {
	return useCatalogWrite(
		({ companyId }, { id, archived }: { id: string; archived: boolean }, key) =>
			setDishArchived(companyId, id, archived, key),
		dishDetail
	);
}
