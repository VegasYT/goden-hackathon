import { deliveryPath, type Delivery } from '@entities/delivery';
import { apiClient, idempotencyHeader } from '@shared/api';

import type { QuarantineResolutionBody } from '../model/quarantine';

export async function resolveQuarantine(
	pointId: string,
	deliveryId: string,
	itemId: string,
	body: QuarantineResolutionBody,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Delivery>(
			`${deliveryPath(pointId, deliveryId)}/items/${encodeURIComponent(itemId)}/quarantine-resolution`,
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
