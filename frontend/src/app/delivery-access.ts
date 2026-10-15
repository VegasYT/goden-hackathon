import type { QueryClient } from '@tanstack/react-query';
import { notFound, redirect } from '@tanstack/react-router';

import { deliveryQueryOptions } from '@entities/delivery';
import { findPoint, mayWrite, meQueryOptions } from '@entities/profile';
import { ApiError } from '@shared/api';

/** Загружает поставку в кэш для loader роута; поставки нет на точке — страница «не найдена». */
export async function ensureDelivery(
	queryClient: QueryClient,
	pointId: string,
	deliveryId: string
) {
	try {
		return await queryClient.ensureQueryData(
			deliveryQueryOptions(pointId, deliveryId)
		);
	} catch (error) {
		if (error instanceof ApiError && error.status === 404) {
			throw notFound();
		}

		throw error;
	}
}

/** Loader исправления: без права приёмки, на отключённой точке и у непринятой поставки ведёт на карточку. */
export async function requireCorrection(
	queryClient: QueryClient,
	pointId: string,
	deliveryId: string
) {
	const [profile, delivery] = await Promise.all([
		queryClient.ensureQueryData(meQueryOptions),
		ensureDelivery(queryClient, pointId, deliveryId),
	]);

	if (
		!mayWrite(profile, findPoint(profile, pointId), 'deliveries.approve') ||
		delivery.status !== 'approved'
	) {
		throw redirect({
			to: '/points/$pointId/deliveries/$deliveryId',
			params: { pointId, deliveryId },
			replace: true,
		});
	}
}
