import * as z from 'zod/mini';

import { hourWindows, type HourWindow } from '@shared/lib/datetime';

/** Параметр адреса со значением по умолчанию: неверное и отсутствующее значение заменяются им. */
export function withDefault<T extends z.ZodMiniType>(
	schema: T,
	value: z.output<T>
) {
	return z.catch(z._default(schema, value as never), value as never);
}

/** Пауза в наборе текста поиска, после которой уходит запрос. */
export const searchDelayMs = 300;

/** Самый длинный текст поиска в адресе и в поле поиска. */
export const searchTextMaxLength = 200;

/** Текст в адресе, по умолчанию пустой. Число в адресе роутер разбирает как число, поэтому оно приводится к строке. */
export const textField = () =>
	withDefault(z.coerce.string().check(z.maxLength(searchTextMaxLength)), '');

/** Варианты числа строк на странице списка. */
export const pageSizes = [5, 10, 25, 50] as const;

export type PageSize = (typeof pageSizes)[number];

/** Страница и число строк в адресе списка: по умолчанию первая страница по 10 строк, размер — только из `pageSizes`. */
export const pageFields = {
	page: withDefault(z.int().check(z.minimum(1)), 1),
	size: withDefault(z.literal(pageSizes), 10),
};

/** `limit` и `offset` запроса страницы списка по странице и числу строк из адреса. */
export function toPageParams({ page, size }: { page: number; size: number }) {
	return { limit: size, offset: (page - 1) * size };
}

/** Окно в часах из `hourWindows` в адресе; другое значение заменяется окном по умолчанию. */
export const hourWindowField = (value: HourWindow) =>
	withDefault(z.literal(hourWindows), value);
