import { describe, expect, it } from 'vitest';

import { rateError } from './rate';

describe('rateError', () => {
	it.each([
		[0, undefined],
		[600, undefined],
		[601, 'Не больше 600'],
		[-1, 'Не меньше 0'],
		[2.5, 'Только целое число'],
		[null, 'Укажите число'],
	])('%s → %s', (value, error) => {
		expect(rateError(value)).toBe(error);
	});
});
