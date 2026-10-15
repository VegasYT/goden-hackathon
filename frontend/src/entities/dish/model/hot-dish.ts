import { remainingLabel } from '@shared/lib/expiry';

import type { HotDish } from '../api/hot-dishes';
import { portionsText } from './dish';

/** Подпись горячего предложения: комментарий закрепившего, продукт под риском и оставшийся срок ближайшей партии; без них — `undefined`. */
export function hotCaption(dish: HotDish, now: Date): string | undefined {
	const [reason] = dish.reasons;
	const parts = [
		dish.manual?.comment && `«${dish.manual.comment}»`,
		reason &&
			`${reason.product.name} · ${remainingLabel(reason.expires_at, now)}`,
	].filter(Boolean);

	return parts.length > 0 ? parts.join(' · ') : undefined;
}

/** Сколько порций стоит продать; у закреплённого блюда без продуктов под риском — `undefined`. */
export function suggestedPortionsText(dish: HotDish): string | undefined {
	return dish.reasons.length > 0
		? portionsText(dish.suggested_portions)
		: undefined;
}
