import { deliveryPath, type Delivery } from '@entities/delivery';
import { apiClient, idempotencyHeader } from '@shared/api';

import type { DeliveryApproval } from '../model/decision';

export async function approveDelivery(
	pointId: string,
	deliveryId: string,
	body: DeliveryApproval,
	idempotencyKey: string
) {
	return (
		await apiClient.post<Delivery>(
			`${deliveryPath(pointId, deliveryId)}/approval`,
			body,
			idempotencyHeader(idempotencyKey)
		)
	).data;
}
