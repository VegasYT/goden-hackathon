import { companyDishesQueryOptions } from '@entities/dish';
import { productListQueryOptions } from '@entities/product';

import { dishFilterParams, type DishFilter } from '../model/search';

const countPage = { limit: 1, offset: 0 };

/** Число действующих или архивных продуктов: читается `total` страницы из одного продукта. */
export const productCountQueryOptions = (
	companyId: string,
	archived: boolean
) =>
	productListQueryOptions(companyId, {
		...(archived ? { status: 'archived' } : {}),
		...countPage,
	});

/** Число блюд по фильтру вкладки «Блюда». */
export const dishCountQueryOptions = (companyId: string, filter: DishFilter) =>
	companyDishesQueryOptions(companyId, {
		...dishFilterParams(filter),
		...countPage,
	});
