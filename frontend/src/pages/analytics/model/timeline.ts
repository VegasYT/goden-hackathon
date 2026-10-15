import { addDays, startOfDay } from 'date-fns';

import { hourMs } from '@shared/lib/datetime';

/** Деление шкалы: полночь и её место на шкале в процентах. */
export interface TimelineTick {
	at: Date;
	share: number;
}

/** Место момента на шкале горизонта в процентах, от 0 до 100. */
export function timelineShare(at: string, now: Date, horizonHours: number) {
	const share =
		((new Date(at).getTime() - now.getTime()) / (horizonHours * hourMs)) * 100;

	return Math.min(100, Math.max(0, share));
}

/** Полуночи внутри горизонта: деления шкалы прогноза. */
export function timelineTicks(now: Date, horizonHours: number): TimelineTick[] {
	const end = now.getTime() + horizonHours * hourMs;
	const ticks: TimelineTick[] = [];

	for (
		let midnight = addDays(startOfDay(now), 1);
		midnight.getTime() < end;
		midnight = addDays(midnight, 1)
	) {
		ticks.push({
			at: midnight,
			share: timelineShare(midnight.toISOString(), now, horizonHours),
		});
	}

	return ticks;
}

// Ширина подписей над шкалой в пикселях с запасом: «сейчас» и «17 окт.» шрифтом caption.
const nowLabelPx = 48;
const tickLabelPx = 46;
const labelGapPx = 8;

/** Деления, подписи которых помещаются над шкалой шириной `widthPx` и не наезжают на «сейчас» и друг на друга. */
export function labeledTicks(ticks: TimelineTick[], widthPx: number) {
	const half = tickLabelPx / 2;
	let freeFrom = nowLabelPx + labelGapPx;

	return ticks.filter((tick) => {
		const center = (tick.share / 100) * widthPx;
		const fits = center - half >= freeFrom && center + half <= widthPx;

		if (fits) {
			freeFrom = center + half + labelGapPx;
		}

		return fits;
	});
}
