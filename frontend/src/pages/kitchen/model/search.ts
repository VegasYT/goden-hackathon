import * as z from 'zod/mini';

import { textField, withDefault } from '@shared/lib/search-params';

/** Поиск и сортировка меню в адресе. */
export const kitchenSearchSchema = z.object({
	q: textField(),
	sort: withDefault(z.enum(['name', 'portions']), 'name'),
});

export type KitchenSearch = z.output<typeof kitchenSearchSchema>;
export type KitchenSort = KitchenSearch['sort'];

/** Значения по умолчанию: их нет в адресе. */
export const kitchenSearchDefaults = kitchenSearchSchema.parse({});
