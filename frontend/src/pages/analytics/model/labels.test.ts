import { describe, expect, it } from 'vitest';

import type { StopListForecast } from '../api/analytics';
import {
	forecastReason,
	formatDuration,
	isSoon,
	momentLabel,
	rateLabel,
	untilLabel,
} from './labels';

const nbsp = ' ';
const now = new Date(2026, 9, 14, 17, 30);
const after = (minutes: number) =>
	new Date(now.getTime() + minutes * 60_000).toISOString();

describe('подписи прогноза', () => {
	it('длительность', () => {
		expect(formatDuration(30_000)).toBe('меньше минуты');
		expect(formatDuration(40 * 60_000)).toBe('40 мин');
		expect(formatDuration(250 * 60_000)).toBe('4 ч 10 мин');
		expect(formatDuration(3 * 60 * 60_000)).toBe('3 ч');
		expect(formatDuration(82_860_000)).toBe('23 ч 1 мин');
		expect(formatDuration(31 * 60 * 60_000)).toBe('1 день 7 ч');
		expect(formatDuration(48 * 60 * 60_000)).toBe('2 дня');
	});

	it('срок до момента и наступивший момент', () => {
		expect(untilLabel(after(250), now)).toBe('через 4 ч 10 мин');
		expect(untilLabel(after(0.5), now)).toBe('скоро');
		expect(untilLabel(after(-1), now)).toBe('скоро');
	});

	it('момент словами сегодня, завтра или датой', () => {
		expect(momentLabel(after(250), now)).toBe('сегодня в 21:40');
		expect(momentLabel(after(13 * 60 + 38), now)).toBe('завтра в 07:08');
		expect(momentLabel(after(31 * 60 + 21), now)).toBe('16 октября в 00:51');
	});

	it('выделяется момент не позже чем через сутки', () => {
		expect(isSoon(after(24 * 60), now)).toBe(true);
		expect(isSoon(after(24 * 60 + 1), now)).toBe(false);
	});

	it('расход в час округляется по единице', () => {
		expect(rateLabel(290.4, 'g')).toBe(`около 290${nbsp}г в час`);
		expect(rateLabel(1250, 'ml')).toBe(`около 1,3${nbsp}л в час`);
		expect(rateLabel(135_360, 'g')).toBe(`около 135,4${nbsp}кг в час`);
		expect(rateLabel(12.4, 'pcs')).toBe(`около 12${nbsp}шт в час`);
		expect(rateLabel(0.5, 'pcs')).toBe(`около 0,5${nbsp}шт в час`);
		expect(rateLabel(0.3, 'g')).toBe(`меньше 1${nbsp}г в час`);
	});

	it('причина: расход или истечение срока партии', () => {
		const item: StopListForecast = {
			dish: { id: 'd', name: 'Брускетта' },
			limiting_product: {
				id: 'p',
				company_id: 'c',
				archived: false,
				name: 'Томаты',
				category: 'Овощи',
				unit: 'g',
				shelf_life_days: 7,
			},
			expected_at: after(250),
			available_quantity: 1200,
			required_quantity: 80,
			consumption_per_hour: 290,
			cause: 'consumption',
		};

		expect(forecastReason(item)).toBe(
			`осталось 1,2${nbsp}кг · около 290${nbsp}г в час · 80${nbsp}г на порцию`
		);
		expect(forecastReason({ ...item, cause: 'expiry' })).toBe(
			`осталось 1,2${nbsp}кг · истечёт срок партии · 80${nbsp}г на порцию`
		);
	});
});
