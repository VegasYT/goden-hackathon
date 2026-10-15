import { describe, expect, it } from 'vitest';

import { formatEventTime } from './datetime';

describe('formatEventTime', () => {
	const now = new Date(2026, 9, 15, 14, 0);

	it('сегодня — только время', () => {
		expect(formatEventTime(new Date(2026, 9, 15, 10, 42), now)).toBe('10:42');
	});

	it('в другой день — дата и время, в другом году — только дата', () => {
		expect(formatEventTime(new Date(2026, 9, 14, 18, 5), now)).toBe(
			'14.10, 18:05'
		);
		expect(formatEventTime(new Date(2025, 9, 15, 10, 42), now)).toBe(
			'15.10.2025'
		);
	});
});
