import { eachDayOfInterval, format } from 'date-fns';

import type { Product } from '@entities/product';

import type { WriteOffDay } from '../api/analytics';
import { lastDayOf, type Period } from './search';

/** Столбцы графика: дни периода и две части каждого дня. */
export interface DayBars {
	days: Date[];
	/** Списания по истечению срока. */
	expired: number[];
	/** Порча и остальные причины. */
	other: number[];
}

/**
 * Столбцы за каждый день периода: число операций, а с выбранным продуктом - его количество.
 * Дни без списаний остаются нулями.
 */
export function dayBars(
	items: WriteOffDay[],
	period: Period,
	now: Date,
	product?: string
): DayBars {
	const days = eachDayOfInterval({
		start: period.start,
		end: lastDayOf(period, now),
	});
	const index = new Map(
		days.map((day, position) => [format(day, 'yyyy-MM-dd'), position])
	);
	const expired = days.map(() => 0);
	const other = days.map(() => 0);

	for (const item of items) {
		const position = index.get(item.day);

		if (position === undefined || (product && item.product.id !== product)) {
			continue;
		}

		const bars = item.reason === 'expired' ? expired : other;

		bars[position] += product ? item.quantity : item.operations;
	}

	return { days, expired, other };
}

/** Продукты, списанные за период, по названию: варианты выбора на графике. */
export function writtenOffProducts(items: WriteOffDay[]): Product[] {
	const products = new Map(
		items.map((item) => [item.product.id, item.product])
	);

	return [...products.values()].sort((a, b) =>
		a.name.localeCompare(b.name, 'ru')
	);
}
