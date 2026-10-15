import { describe, expect, it } from 'vitest';

import {
	analyticsSearchSchema,
	periodLabel,
	periodOf,
	toPeriodParams,
} from './search';

const now = new Date(2026, 9, 14, 17, 30);

describe('период аналитики', () => {
	it('7 дней - сегодня и шесть предыдущих дней с полуночи, до текущего момента', () => {
		const period = periodOf(analyticsSearchSchema.parse({}), now);

		expect(toPeriodParams(period)).toEqual({
			from: new Date(2026, 9, 8).toISOString(),
			to: undefined,
		});
		expect(periodLabel(period, now)).toBe('8–14 октября');
	});

	it('30 дней захватывают прошлый месяц', () => {
		const period = periodOf(
			analyticsSearchSchema.parse({ period: '30d' }),
			now
		);

		expect(period.start).toEqual(new Date(2026, 8, 15));
		expect(periodLabel(period, now)).toBe('15 сентября – 14 октября');
	});

	it('свой период включает оба дня целиком', () => {
		const period = periodOf(
			analyticsSearchSchema.parse({
				period: 'custom',
				from: '2026-10-01',
				to: '2026-10-14',
			}),
			now
		);

		expect(toPeriodParams(period)).toEqual({
			from: new Date(2026, 9, 1).toISOString(),
			to: new Date(2026, 9, 15).toISOString(),
		});
		expect(periodLabel(period, now)).toBe('1–14 октября');
	});

	it('неполный, перевёрнутый или испорченный свой период заменяется на 7 дней', () => {
		const searches = [
			{ period: 'custom', from: '2026-10-01' },
			{ period: 'custom', from: '2026-10-14', to: '2026-10-01' },
			{ period: 'custom', from: '1 октября', to: '2026-10-14' },
		];

		for (const search of searches) {
			expect(periodOf(analyticsSearchSchema.parse(search), now)).toEqual({
				choice: '7d',
				start: new Date(2026, 9, 8),
			});
		}
	});

	it('неверные значения адреса заменяются умолчаниями', () => {
		expect(
			analyticsSearchSchema.parse({
				horizon: 100,
				period: 'year',
				writeOffs: 'days',
			})
		).toEqual({
			horizon: 168,
			period: '7d',
			from: undefined,
			to: undefined,
			writeOffs: 'products',
			dayProduct: undefined,
		});
	});
});
