import { afterEach, describe, expect, it, vi } from 'vitest';

import { readStorage, writeStorage } from './storage';

describe('storage', () => {
	afterEach(() => vi.restoreAllMocks());

	it('при ошибке localStorage возвращает null и не падает', () => {
		vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
			throw new Error('denied');
		});
		vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
			throw new Error('denied');
		});

		expect(() => writeStorage('k', 'v')).not.toThrow();
		expect(readStorage('k')).toBeNull();
	});
});
