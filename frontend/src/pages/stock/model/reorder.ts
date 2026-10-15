import type { DeliveryRecommendation } from '../api/recommendations';

/** На сколько хватит остатка: «9 ч», «меньше часа»; `null`, если расхода не было или продукта нет. */
export function coverLabel(item: DeliveryRecommendation): string | null {
	if (item.available_quantity === 0 || item.cover_hours === undefined) {
		return null;
	}

	return item.cover_hours < 1
		? 'меньше часа'
		: `${Math.floor(item.cover_hours)} ч`;
}
