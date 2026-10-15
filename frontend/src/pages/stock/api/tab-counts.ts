import {
	expiringBatchesQueryOptions,
	urgentExpiringHours,
} from '@entities/stock';

import { reorderSearchDefaults, toRecommendationPage } from '../model/search';
import { deliveryRecommendationsQueryOptions } from './recommendations';

/** Число на вкладке «Сроки»: партии, истекающие в ближайшие сутки или уже просроченные. */
export const urgentExpiringQueryOptions = (pointId: string) =>
	expiringBatchesQueryOptions(pointId, {
		within_hours: urgentExpiringHours,
		limit: 1,
		offset: 0,
	});

/** Число на вкладке «Пора заказать»: тот же запрос, что первая страница вкладки по умолчанию. */
export const reorderCountQueryOptions = (pointId: string) =>
	deliveryRecommendationsQueryOptions(
		pointId,
		toRecommendationPage(reorderSearchDefaults)
	);
