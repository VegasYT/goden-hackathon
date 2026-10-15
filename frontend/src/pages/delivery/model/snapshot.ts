import type { DeliveryItem } from '@entities/delivery';

/**
 * Принятое и отклонённое без исправлений: приёмка плюс решения по карантину.
 * Если текущее от него отличается, позицию исправляли.
 */
export function withoutCorrections(
	item: DeliveryItem
): { accepted: number; rejected: number } | undefined {
	if (!item.approved) {
		return undefined;
	}

	return item.quarantine_resolutions.reduce(
		(sum, resolution) => ({
			accepted: sum.accepted + resolution.accepted_quantity,
			rejected: sum.rejected + resolution.rejected_quantity,
		}),
		{
			accepted: item.approved.accepted_quantity,
			rejected: item.approved.rejected_quantity,
		}
	);
}
