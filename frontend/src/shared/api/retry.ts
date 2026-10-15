import { ApiError } from './api-error';

const maxRetries = 3;

/** `retry` запросов: повторяет обрыв связи и ответы 5xx, ответ 4xx сразу становится ошибкой. */
export function shouldRetry(failureCount: number, error: unknown) {
	const status = error instanceof ApiError ? error.status : undefined;

	return failureCount < maxRetries && (status === undefined || status >= 500);
}
