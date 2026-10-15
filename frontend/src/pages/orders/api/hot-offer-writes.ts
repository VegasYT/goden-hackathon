import { useMutation, useQueryClient } from '@tanstack/react-query';

import { hotDishKeys, setHotOffer, type HotOfferChange } from '@entities/dish';
import { useIdempotencyKey } from '@shared/lib/idempotency';

/** Меняет режим горячего предложения; порядок списка считает сервер, поэтому после записи предложения перечитываются. */
export function useSetHotOffer(pointId: string) {
	const keyFor = useIdempotencyKey();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: ({
			dishId,
			change,
		}: {
			dishId: string;
			change: HotOfferChange;
		}) => setHotOffer(pointId, dishId, change, keyFor({ dishId, change })),
		onSuccess: () =>
			queryClient.invalidateQueries({ queryKey: hotDishKeys.all(pointId) }),
	});
}
