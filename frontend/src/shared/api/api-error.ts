import { isAxiosError } from 'axios';

import type { components } from './schema.gen';

type Problem = components['schemas']['Problem'];

/** Ошибка поля запроса: текст для показа, а также вид и предел, если сервер их различает. */
export interface FieldError {
	message: string;
	code?: string;
	limit?: number;
}

const noConnectionMessage =
	'Нет связи с сервером. Проверьте подключение и повторите';
const unexpectedResponseMessage =
	'Сервер вернул неожиданный ответ. Повторите позже';

/** Ошибка запроса к API; `message` готов для показа пользователю. */
export class ApiError extends Error {
	/** HTTP-статус; `undefined`, если ответа не было. */
	readonly status?: number;
	/** Вид ошибки из `problem+json`. */
	readonly type?: string;
	/** Ошибки полей по имени поля запроса. */
	readonly fieldErrors: Readonly<Record<string, FieldError>>;

	constructor(
		message: string,
		details: {
			status?: number;
			type?: string;
			fieldErrors?: Record<string, FieldError>;
		} = {}
	) {
		super(message);
		this.name = 'ApiError';
		this.status = details.status;
		this.type = details.type;
		this.fieldErrors = details.fieldErrors ?? {};
	}
}

function isProblem(data: unknown): data is Problem {
	return (
		typeof data === 'object' &&
		data !== null &&
		typeof (data as Problem).title === 'string'
	);
}

/** Превращает ошибку запроса axios в `ApiError`. */
export function toApiError(error: unknown): ApiError {
	if (error instanceof ApiError) {
		return error;
	}

	const response = isAxiosError(error) ? error.response : undefined;

	if (!response) {
		return new ApiError(noConnectionMessage);
	}

	const data: unknown = response.data;

	if (!isProblem(data)) {
		return new ApiError(unexpectedResponseMessage, {
			status: response.status,
		});
	}

	const fieldErrors: Record<string, FieldError> = {};

	for (const { field, message, code, limit } of data.errors ?? []) {
		fieldErrors[field] ??= { message, code, limit };
	}

	return new ApiError(data.detail || data.title, {
		status: response.status,
		type: data.type,
		fieldErrors,
	});
}
