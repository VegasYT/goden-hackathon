import { describe, expect, it } from 'vitest';

import { labeledTicks, timelineShare, timelineTicks } from './timeline';

const now = new Date(2026, 9, 14, 18, 0);
const after = (hours: number) =>
	new Date(now.getTime() + hours * 3_600_000).toISOString();

describe('шкала прогноза', () => {
	it('доля момента в горизонте не выходит за края шкалы', () => {
		expect(timelineShare(after(36), now, 72)).toBe(50);
		expect(timelineShare(after(-1), now, 72)).toBe(0);
		expect(timelineShare(after(100), now, 72)).toBe(100);
	});

	it('деления - полуночи внутри горизонта', () => {
		expect(timelineTicks(now, 72).map((tick) => tick.at)).toEqual([
			new Date(2026, 9, 15),
			new Date(2026, 9, 16),
			new Date(2026, 9, 17),
		]);
		expect(timelineTicks(now, 72)[0].share).toBeCloseTo((6 / 72) * 100);
	});
});

describe('labeledTicks', () => {
	const tick = (share: number) => ({ at: new Date(2026, 9, 16), share });

	it('скрывает подпись, которая наезжает на «сейчас»', () => {
		expect(labeledTicks([tick(10), tick(40)], 300)).toEqual([tick(40)]);
	});

	it('на узкой шкале оставляет подписи через одну, чтобы они не слипались', () => {
		const week = [14, 28, 42, 56, 70, 84, 98].map(tick);

		expect(labeledTicks(week, 300).map((t) => t.share)).toEqual([28, 56, 84]);
	});

	it('скрывает подпись, которая выходит за правый край', () => {
		expect(labeledTicks([tick(97)], 300)).toEqual([]);
	});
});
