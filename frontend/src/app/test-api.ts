import { vi, type MockInstance } from 'vitest';

import { apiClient } from '@shared/api';

type Params = Record<string, unknown>;

/** Адрес запроса: строка сравнивается с концом адреса, выражение — целиком. */
type UrlMatch = string | RegExp;

/** Ключ идемпотентности из вызова `apiClient.post` или `put`. */
export function idempotencyKeyOf(call: unknown[]) {
	return (call[2] as { headers: Record<string, string> }).headers[
		'Idempotency-Key'
	];
}

type GetHandler = (url: string, params: Params) => unknown | Promise<unknown>;

/** Подменяет `GET` к API: обработчик получает адрес и параметры запроса, его ответ — тело. */
export function mockGet(handler: GetHandler) {
	return vi.spyOn(apiClient, 'get').mockImplementation(async (url, config) => ({
		data: await handler(url, (config?.params ?? {}) as Params),
	}));
}

/** Вызовы подменённого запроса к адресу `match`. */
export function requestsTo(mock: MockInstance, match: UrlMatch) {
	return mock.mock.calls.filter(([url]) =>
		typeof match === 'string'
			? String(url).endsWith(match)
			: match.test(String(url))
	);
}

/** Параметры подменённых `GET` к адресу `match` в порядке запросов. */
export function paramsOf(get: MockInstance, match: UrlMatch): Params[] {
	return requestsTo(get, match).map(
		([, config]) => (config as { params?: Params } | undefined)?.params ?? {}
	);
}

/** Параметры последнего подменённого `GET` к адресу `match`. */
export function lastParams(get: MockInstance, match: UrlMatch) {
	return paramsOf(get, match).at(-1);
}

/** Параметры последнего запроса списка, без счётчиков вкладок (`limit: 1`). */
export function lastListParams(get: MockInstance, match: UrlMatch) {
	return paramsOf(get, match)
		.filter((params) => params.limit !== 1)
		.at(-1);
}
