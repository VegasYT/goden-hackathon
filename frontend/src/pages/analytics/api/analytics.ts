import { queryOptions, type QueryClient } from '@tanstack/react-query';

import { getData, pointPath, type components } from '@shared/api';
import { browserTimeZone } from '@shared/lib/datetime';
import { keepPointData, pointKey } from '@shared/lib/query';

import {
	periodOf,
	toPeriodParams,
	type AnalyticsSearch,
} from '../model/search';

export type StopListForecast = components['schemas']['StopListForecast'];
export type WriteOffStatistic = components['schemas']['WriteOffStatistic'];
export type WriteOffDay = components['schemas']['WriteOffDay'];

/** Границы периода статистики; без `to` сервер считает до текущего момента. */
export interface PeriodParams {
	from: string;
	to?: string;
}

/** Ключи запросов аналитики точки; вся аналитика сбрасывается по `analyticsKeys.all`. */
export const analyticsKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'analytics'] as const,
	forecast: (pointId: string, horizonHours: number) =>
		[...analyticsKeys.all(pointId), 'forecast', horizonHours] as const,
	shortages: (pointId: string, period: PeriodParams) =>
		[...analyticsKeys.all(pointId), 'shortages', period] as const,
	writeOffs: (pointId: string, period: PeriodParams) =>
		[...analyticsKeys.all(pointId), 'write-offs', period] as const,
	writeOffDays: (pointId: string, period: PeriodParams) =>
		[...analyticsKeys.all(pointId), 'write-off-days', period] as const,
};

const analyticsPath = (pointId: string, report: string) =>
	`${pointPath(pointId)}/analytics/${report}`;

/** Блюда, которые попадут в стоп-лист в горизонте, от ближайшего. */
export function forecastQueryOptions(pointId: string, horizonHours: number) {
	return queryOptions({
		queryKey: analyticsKeys.forecast(pointId, horizonHours),
		queryFn: async (context) =>
			(
				await getData<components['schemas']['StopListForecasts']>(
					analyticsPath(pointId, 'stop-list-forecast'),
					{ horizon_hours: horizonHours }
				)(context)
			).items,
		placeholderData: keepPointData(pointId),
	});
}

/** Продукты по времени, которое блюда с ними простояли в стоп-листе за период. */
export function shortagesQueryOptions(pointId: string, period: PeriodParams) {
	return queryOptions({
		queryKey: analyticsKeys.shortages(pointId, period),
		queryFn: async (context) =>
			(
				await getData<components['schemas']['ShortageStatistics']>(
					analyticsPath(pointId, 'shortages'),
					period
				)(context)
			).items,
		placeholderData: keepPointData(pointId),
	});
}

/** Списания за период парами "продукт и причина". */
export function writeOffsQueryOptions(pointId: string, period: PeriodParams) {
	return queryOptions({
		queryKey: analyticsKeys.writeOffs(pointId, period),
		queryFn: async (context) =>
			(
				await getData<components['schemas']['WriteOffStatistics']>(
					analyticsPath(pointId, 'write-offs'),
					period
				)(context)
			).items,
		placeholderData: keepPointData(pointId),
	});
}

/** Часовой пояс браузера: в нём сервер считает границы дней. */
const timeZone = browserTimeZone();

/** Списания за период по дням, продуктам и причинам; дней без списаний нет. */
export function writeOffDaysQueryOptions(
	pointId: string,
	period: PeriodParams
) {
	return queryOptions({
		queryKey: analyticsKeys.writeOffDays(pointId, period),
		queryFn: async (context) =>
			(
				await getData<components['schemas']['WriteOffDays']>(
					analyticsPath(pointId, 'write-offs/days'),
					{ ...period, time_zone: timeZone }
				)(context)
			).items,
		placeholderData: keepPointData(pointId),
	});
}

/** Начинает загрузку всех блоков аналитики за горизонт и период из адреса. */
export function prefetchAnalytics(
	queryClient: QueryClient,
	pointId: string,
	search: Pick<AnalyticsSearch, 'horizon' | 'period' | 'from' | 'to'>
) {
	const period = toPeriodParams(periodOf(search, new Date()));

	void queryClient.prefetchQuery(forecastQueryOptions(pointId, search.horizon));
	void queryClient.prefetchQuery(shortagesQueryOptions(pointId, period));
	void queryClient.prefetchQuery(writeOffsQueryOptions(pointId, period));
	void queryClient.prefetchQuery(writeOffDaysQueryOptions(pointId, period));
}
