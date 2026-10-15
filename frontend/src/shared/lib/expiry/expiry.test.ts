import { describe, expect, it } from 'vitest';

import { expiryStatus, remainingLabel } from './expiry';

const now = new Date('2026-10-12T14:32:00Z');
const later = (ms: number) => new Date(now.getTime() + ms);
const hour = 3_600_000;
const day = 24 * hour;

describe('статус срока', () => {
	it.each([
		['ровно в момент проверки', 0, 'expired'],
		['час назад', -hour, 'expired'],
		['через 17 часов', 17 * hour, 'soon'],
		['ровно через сутки', day, 'soon'],
		['через сутки и минуту', day + 60_000, 'ok'],
	] as const)('%s — %s', (_, offset, status) => {
		expect(expiryStatus(later(offset), now)).toBe(status);
	});
});

describe('оставшийся срок', () => {
	it.each([
		[40 * 60_000, 'ещё 40 мин'],
		[17 * hour + 59 * 60_000, 'ещё 17 ч'],
		[day, 'ещё 1 день'],
		[3 * day + hour, 'ещё 3 дня'],
		[12 * day, 'ещё 12 дней'],
		[71 * day, 'ещё 2 месяца'],
		[171 * day, 'ещё 5 месяцев'],
		[0, ''],
	])('через %i мс — «%s»', (offset, label) => {
		expect(remainingLabel(later(offset), now)).toBe(label);
	});
});
