import { queryOptions } from '@tanstack/react-query';

import {
	apiClient,
	getData,
	idempotencyHeader,
	pointPath,
	type components,
} from '@shared/api';
import { keepPointData, pointKey } from '@shared/lib/query';

export type HotDish = components['schemas']['DishRecommendation'];
export type HotReason = components['schemas']['RecommendationReason'];
type HotDishPage = components['schemas']['DishRecommendationPage'];
export type HotOfferChange = components['schemas']['HotOfferChange'];
export type HotOfferSetting = components['schemas']['HotOfferSetting'];

/** Горизонт горячих предложений по умолчанию, в часах: на вкладке «Горячие предложения» и на сводке. */
export const defaultHotHorizonHours = 72;

export interface HotDishParams {
	horizon_hours: number;
	limit: number;
	offset: number;
}

/** Ключи горячих предложений точки; все сбрасываются по `hotDishKeys.all`. */
export const hotDishKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'hot-dishes'] as const,
	page: (pointId: string, params: HotDishParams) =>
		[...hotDishKeys.all(pointId), params] as const,
	settings: (pointId: string) =>
		[...hotDishKeys.all(pointId), 'settings'] as const,
};

const hotDishesPath = (pointId: string) =>
	`${pointPath(pointId)}/recommendations/dishes`;

/** Страница горячих предложений от самого приоритетного. */
export function hotDishesQueryOptions(pointId: string, params: HotDishParams) {
	return queryOptions({
		queryKey: hotDishKeys.page(pointId, params),
		queryFn: getData<HotDishPage>(hotDishesPath(pointId), params),
		placeholderData: keepPointData(pointId),
	});
}

/** Блюда, закреплённые вручную или скрытые сегодня, в порядке изменения. */
export function hotOfferSettingsQueryOptions(pointId: string) {
	return queryOptions({
		queryKey: hotDishKeys.settings(pointId),
		queryFn: async (context) =>
			(
				await getData<components['schemas']['HotOfferSettings']>(
					`${hotDishesPath(pointId)}/settings`
				)(context)
			).items,
	});
}

/** Закрепляет блюдо, скрывает его до конца дня или возвращает расчёту. */
export async function setHotOffer(
	pointId: string,
	dishId: string,
	change: HotOfferChange,
	idempotencyKey: string
) {
	return (
		await apiClient.put<HotOfferSetting>(
			`${hotDishesPath(pointId)}/${encodeURIComponent(dishId)}`,
			change,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
