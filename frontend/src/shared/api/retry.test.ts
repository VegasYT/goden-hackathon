import { describe, expect, it } from 'vitest';

import { ApiError } from './api-error';
import { shouldRetry } from './retry';

const failed = (status?: number) => new ApiError('Ошибка', { status });

describe('shouldRetry', () => {
	it.each([403, 404, 422])('ответ %i не повторяется', (status) => {
		expect(shouldRetry(0, failed(status))).toBe(false);
	});

	it('ошибка сервера и обрыв связи повторяются', () => {
		expect(shouldRetry(0, failed(500))).toBe(true);
		expect(shouldRetry(0, failed())).toBe(true);
	});

	it('после трёх неудач повторов больше нет', () => {
		expect(shouldRetry(3, failed(503))).toBe(false);
	});
});
