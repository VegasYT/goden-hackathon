import { hotDishesQueryOptions } from '@entities/dish';

import { hotSearchDefaults, toHotDishParams } from '../model/search';

/** Первая страница предложений по умолчанию: её число стоит на вкладке, её блюда отмечены в форме заказа. */
export const defaultHotDishesQueryOptions = (pointId: string) =>
	hotDishesQueryOptions(pointId, toHotDishParams(hotSearchDefaults));
