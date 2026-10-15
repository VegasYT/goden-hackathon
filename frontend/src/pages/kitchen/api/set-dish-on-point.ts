import { useMutation, useQueryClient } from '@tanstack/react-query';

import {
	dishKeys,
	setDishOnPoint,
	type DishAvailability,
	type PointDish,
} from '@entities/dish';
import { useIdempotencyKey } from '@shared/lib/idempotency';

/** Выключает или включает блюдо на точке и обновляет меню. */
export function useSetDishOnPoint(pointId: string, dishId: string) {
	const queryClient = useQueryClient();
	const keyFor = useIdempotencyKey();

	return useMutation({
		mutationFn: (enabled: boolean) =>
			setDishOnPoint(pointId, dishId, enabled, keyFor({ enabled })),
		onSuccess: (updated) => {
			queryClient.setQueryData<PointDish[]>(
				dishKeys.dishes(pointId),
				(dishes) =>
					dishes?.map((dish) =>
						dish.dish_id === updated.dish_id ? updated : dish
					)
			);

			// Выключенное блюдо просто уходит из меню; порции включённого считает сервер.
			if (!updated.enabled) {
				queryClient.setQueryData<DishAvailability[]>(
					dishKeys.menu(pointId),
					(menu) => menu?.filter((item) => item.dish.id !== updated.dish_id)
				);

				return;
			}

			return queryClient.invalidateQueries({
				queryKey: dishKeys.menu(pointId),
			});
		},
	});
}
