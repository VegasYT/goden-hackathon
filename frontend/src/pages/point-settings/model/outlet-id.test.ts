import { describe, expect, it } from 'vitest';

import { parseOutletId } from './outlet-id';

describe('parseOutletId', () => {
	it.each([
		['42', 42],
		[' 42 ', 42],
		['42а', null],
		['0', null],
		['-3', null],
		['', null],
	])('«%s» — %s', (text, expected) => {
		expect(parseOutletId(text)).toBe(expected);
	});
});
