import * as z from 'zod/mini';

import { defaultHotHorizonHours, type HotDishParams } from '@entities/dish';
import {
	hourWindowField,
	pageFields,
	toPageParams,
	withDefault,
} from '@shared/lib/search-params';

import type { OrderListParams } from '../api/orders';

/** Состояние истории заказов в адресе. */
export const historySearchSchema = z.object({
	status: withDefault(
		z.enum(['all', 'accepted', 'rejected', 'cancelled']),
		'all'
	),
	...pageFields,
});

export type HistorySearch = z.output<typeof historySearchSchema>;
export type OrderStatusFilter = HistorySearch['status'];

/** Значения по умолчанию: их нет в адресе. */
export const historySearchDefaults = historySearchSchema.parse({});

export function toOrderListParams(search: HistorySearch): OrderListParams {
	return {
		status: search.status === 'all' ? undefined : search.status,
		...toPageParams(search),
	};
}

/** Состояние вкладки «Горячие предложения» в адресе. */
export const hotSearchSchema = z.object({
	horizon: hourWindowField(defaultHotHorizonHours),
	...pageFields,
});

export type HotSearch = z.output<typeof hotSearchSchema>;

export const hotSearchDefaults = hotSearchSchema.parse({});

export function toHotDishParams(search: HotSearch): HotDishParams {
	return { horizon_hours: search.horizon, ...toPageParams(search) };
}
