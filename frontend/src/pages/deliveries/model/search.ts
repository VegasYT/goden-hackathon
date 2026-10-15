import * as z from 'zod/mini';

import {
	pageFields,
	textField,
	toPageParams,
	withDefault,
} from '@shared/lib/search-params';

import type { DeliveryListParams } from '@entities/delivery';

/** Состояние списка в адресе. */
export const deliveryListSearchSchema = z.object({
	status: withDefault(z.enum(['all', 'registered', 'approved']), 'all'),
	number: textField(),
	sort: withDefault(z.enum(['-delivered_at', 'delivered_at']), '-delivered_at'),
	...pageFields,
	/** Открытая боковая панель списка. */
	panel: z.catch(z.optional(z.enum(['quarantine'])), undefined),
});

export type DeliveryListSearch = z.output<typeof deliveryListSearchSchema>;
export type DeliveryStatusFilter = DeliveryListSearch['status'];

/** Значения по умолчанию: их нет в адресе. */
export const deliveryListDefaults = deliveryListSearchSchema.parse({});

export function toListParams(search: DeliveryListSearch): DeliveryListParams {
	return {
		status: search.status === 'all' ? undefined : search.status,
		number: search.number.trim() || undefined,
		sort: search.sort,
		...toPageParams(search),
	};
}
