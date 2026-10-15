import axios from 'axios';

import { getAccessToken } from '@shared/auth';

import { toApiError } from './api-error';

/** Адрес точки в API, от него строятся адреса её разделов: `${pointPath(id)}/stock`. */
export const pointPath = (pointId: string) =>
	`/points/${encodeURIComponent(pointId)}`;

/** Адрес компании в API: `${companyPath(id)}/products`. */
export const companyPath = (companyId: string) =>
	`/companies/${encodeURIComponent(companyId)}`;

/** Настройки запроса записи с ключом идемпотентности. */
export const idempotencyHeader = (idempotencyKey: string) => ({
	headers: { 'Idempotency-Key': idempotencyKey },
});

/** Клиент API: добавляет токен к каждому запросу и отдаёт ошибки как `ApiError`. */
export const apiClient = axios.create({ baseURL: '/api/v1' });

/** `queryFn` чтения: `GET` по адресу с параметрами, результат — тело ответа. */
export const getData =
	<T>(url: string, params?: object) =>
	async ({ signal }: { signal: AbortSignal }) =>
		(await apiClient.get<T>(url, { params, signal })).data;

apiClient.interceptors.request.use(async (config) => {
	config.headers.Authorization = `Bearer ${await getAccessToken()}`;
	return config;
});

apiClient.interceptors.response.use(undefined, (error: unknown) => {
	if (axios.isCancel(error)) {
		return Promise.reject(error);
	}

	return Promise.reject(toApiError(error));
});
