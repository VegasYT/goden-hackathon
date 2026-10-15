import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { getAccessToken } from '@shared/auth';

import { ApiError } from './api-error';
import { apiClient } from './client';

vi.mock('@shared/auth', () => ({ getAccessToken: vi.fn() }));

const adapter = vi.fn();

beforeEach(() => {
	vi.mocked(getAccessToken).mockResolvedValue('fresh-token');
	apiClient.defaults.adapter = adapter;
});

describe('apiClient', () => {
	it('отправляет запрос на /api/v1 с токеном в заголовке', async () => {
		adapter.mockImplementation((config: InternalAxiosRequestConfig) =>
			Promise.resolve({ status: 200, data: [], headers: {}, config })
		);

		await apiClient.get('/products');

		const config = adapter.mock.calls[0][0] as InternalAxiosRequestConfig;

		expect(apiClient.getUri(config)).toBe('/api/v1/products');
		expect(config.headers.Authorization).toBe('Bearer fresh-token');
	});

	it('без токена запрос не отправляет и сообщает, что нет связи', async () => {
		vi.mocked(getAccessToken).mockRejectedValue(new Error('network'));

		const request = apiClient.get('/products');

		await expect(request).rejects.toBeInstanceOf(ApiError);
		await expect(request).rejects.toHaveProperty(
			'message',
			'Нет связи с сервером. Проверьте подключение и повторите'
		);
		expect(adapter).not.toHaveBeenCalled();
	});

	it('отменённый запрос отклоняется отменой, а не ошибкой связи', async () => {
		await expect(
			apiClient.get('/products', { signal: AbortSignal.abort() })
		).rejects.toSatisfy(axios.isCancel);
	});

	it('ответ 401 отдаёт как ошибку с текстом из ответа', async () => {
		adapter.mockImplementation((config: InternalAxiosRequestConfig) =>
			Promise.reject(
				new AxiosError('401', 'ERR_BAD_REQUEST', config, null, {
					status: 401,
					statusText: '',
					data: { type: 'unauthorized', title: 'Токен не принят', status: 401 },
					headers: { 'content-type': 'application/problem+json' },
					config,
				})
			)
		);

		await expect(apiClient.get('/products')).rejects.toMatchObject({
			status: 401,
			message: 'Токен не принят',
		});
	});
});
