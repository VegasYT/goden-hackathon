import { addDays, parseISO, startOfDay } from 'date-fns';
import * as z from 'zod/mini';

import { formatDayRange } from '@shared/lib/datetime';
import { hourWindowField, withDefault } from '@shared/lib/search-params';

import type { PeriodParams } from '../api/analytics';

const dayPattern = /^\d{4}-\d{2}-\d{2}$/;
const dayField = () =>
	z.catch(z.optional(z.string().check(z.regex(dayPattern))), undefined);

/** Состояние аналитики в адресе. */
export const analyticsSearchSchema = z.object({
	/** Горизонт прогноза стоп-листа в часах. */
	horizon: hourWindowField(168),
	period: withDefault(z.enum(['7d', '30d', 'custom']), '7d'),
	/** Первый и последний день своего периода, "гггг-мм-дд". */
	from: dayField(),
	to: dayField(),
	writeOffs: withDefault(z.enum(['products', 'reasons']), 'products'),
	/** Продукт, списания которого показывает график по дням. */
	dayProduct: z.catch(z.optional(z.string()), undefined),
});

export type AnalyticsSearch = z.output<typeof analyticsSearchSchema>;
export type PeriodChoice = AnalyticsSearch['period'];

/** Значения по умолчанию: их нет в адресе. */
export const analyticsSearchDefaults = analyticsSearchSchema.parse({});

/** Период статистики: `end` - полночь после последнего дня, без него - до текущего момента. */
export interface Period {
	choice: PeriodChoice;
	start: Date;
	end?: Date;
}

/** Период из адреса; неполный или перевёрнутый свой период заменяется на 7 дней. */
export function periodOf(
	search: Pick<AnalyticsSearch, 'period' | 'from' | 'to'>,
	now: Date
): Period {
	if (search.period === 'custom' && search.from && search.to) {
		// Дата без времени читается как полночь в часовом поясе браузера.
		const start = parseISO(search.from);
		const last = parseISO(search.to);

		if (start <= last) {
			return { choice: 'custom', start, end: addDays(last, 1) };
		}
	}

	const choice = search.period === '30d' ? '30d' : '7d';
	const days = choice === '30d' ? 30 : 7;

	return { choice, start: addDays(startOfDay(now), 1 - days) };
}

export function toPeriodParams(period: Period): PeriodParams {
	return {
		from: period.start.toISOString(),
		to: period.end?.toISOString(),
	};
}

/** Последний день периода: день перед `end`, без него - сегодня. */
export function lastDayOf(period: Period, now: Date): Date {
	return period.end ? addDays(period.end, -1) : startOfDay(now);
}

/** Дни периода: "8–14 октября", "15 сентября – 14 октября"; год появляется, если он разный. */
export function periodLabel(period: Period, now: Date): string {
	return formatDayRange(period.start, lastDayOf(period, now));
}
