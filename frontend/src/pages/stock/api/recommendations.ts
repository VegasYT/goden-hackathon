import { queryOptions } from '@tanstack/react-query';

import { getData, pointPath, type components } from '@shared/api';
import { keepPointData, pointKey } from '@shared/lib/query';

export type DeliveryRecommendation =
	components['schemas']['DeliveryRecommendation'];
type DeliveryRecommendationPage =
	components['schemas']['DeliveryRecommendationPage'];

/** Параметры расчёта рекомендаций; их же показывает строка над списком. */
export const supplyHours = {
	window_hours: 24,
	cover_hours: 24,
	target_hours: 72,
} as const;

export interface RecommendationPage {
	limit: number;
	offset: number;
}

/** Ключи рекомендаций точки; все сбрасываются по `recommendationKeys.all`. */
export const recommendationKeys = {
	all: (pointId: string) => [...pointKey(pointId), 'recommendations'] as const,
	deliveries: (pointId: string, page: RecommendationPage) =>
		[...recommendationKeys.all(pointId), 'deliveries', page] as const,
};

/** Страница рекомендаций поставок от самых срочных. */
export function deliveryRecommendationsQueryOptions(
	pointId: string,
	page: RecommendationPage
) {
	return queryOptions({
		queryKey: recommendationKeys.deliveries(pointId, page),
		queryFn: getData<DeliveryRecommendationPage>(
			`${pointPath(pointId)}/recommendations/deliveries`,
			{ ...supplyHours, ...page }
		),
		placeholderData: keepPointData(pointId),
	});
}
