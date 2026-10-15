import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';

import { toApiError } from './api-error';

function responseError(status: number, data: unknown, contentType: string) {
	const config = { headers: new AxiosHeaders() };
	const response = {
		status,
		statusText: '',
		data,
		headers: { 'content-type': contentType },
		config,
	} as AxiosResponse;

	return new AxiosError('fail', 'ERR_BAD_RESPONSE', config, null, response);
}

const problem = 'application/problem+json';

describe('toApiError', () => {
	it('показывает detail из problem+json', () => {
		const error = toApiError(
			responseError(
				403,
				{
					type: 'forbidden',
					title: 'Forbidden',
					status: 403,
					detail: 'Для этого действия нужно право inventory.edit',
				},
				problem
			)
		);

		expect(error.message).toBe('Для этого действия нужно право inventory.edit');
		expect(error.status).toBe(403);
	});

	it('без detail показывает title, ошибки полей отдаёт по имени поля', () => {
		const error = toApiError(
			responseError(
				422,
				{
					type: 'validation',
					title: 'Некорректный запрос',
					status: 422,
					errors: [{ field: 'quantity', message: 'Больше нуля' }],
				},
				problem
			)
		);

		expect(error.message).toBe('Некорректный запрос');
		expect(error.fieldErrors).toEqual({
			quantity: { message: 'Больше нуля', code: undefined, limit: undefined },
		});
	});

	it('сохраняет вид ошибки поля и предел рядом с текстом', () => {
		const error = toApiError(
			responseError(
				422,
				{
					type: 'validation-error',
					title: 'Ошибка валидации',
					status: 422,
					errors: [
						{
							field: 'items[a].accepted_quantity',
							message: 'позиция просрочена и не может быть принята',
							code: 'expired',
						},
						{
							field: 'items[b].accepted_quantity',
							message: 'нельзя уменьшить ниже израсходованного',
							code: 'stock-consumed',
							limit: 6500,
						},
						{ field: 'reason', message: 'обязательно' },
					],
				},
				problem
			)
		);

		expect(error.fieldErrors['items[a].accepted_quantity']).toMatchObject({
			message: 'позиция просрочена и не может быть принята',
			code: 'expired',
		});
		expect(error.fieldErrors['items[b].accepted_quantity']).toMatchObject({
			code: 'stock-consumed',
			limit: 6500,
		});
	});

	it('без ответа сообщает о потере связи', () => {
		const error = toApiError(new AxiosError('Network Error', 'ERR_NETWORK'));

		expect(error.message).toBe(
			'Нет связи с сервером. Проверьте подключение и повторите'
		);
	});

	it('на HTML от прокси сообщает о неожиданном ответе', () => {
		const error = toApiError(
			responseError(502, '<html>Bad Gateway</html>', 'text/html')
		);

		expect(error.message).toBe(
			'Сервер вернул неожиданный ответ. Повторите позже'
		);
	});
});
