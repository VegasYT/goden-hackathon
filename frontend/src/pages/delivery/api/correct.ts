import { deliveryPath, type Delivery } from '@entities/delivery';
import { apiClient, idempotencyHeader } from '@shared/api';

import type { ApprovalCorrection } from '../model/correction';

export async function correctDelivery(
	pointId: string,
	deliveryId: string,
	body: ApprovalCorrection,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Delivery>(
			`${deliveryPath(pointId, deliveryId)}/corrections`,
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
