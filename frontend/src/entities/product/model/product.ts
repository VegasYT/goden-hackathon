import { countOf } from '@shared/lib/plural';

import type { Product } from '../api/product-queries';

/** «5 суток». */
export const shelfLifeDays = (days: number) =>
	countOf(days, 'суток', 'суток', 'суток');

/** «хранится до 5 суток»; `undefined`, если срок хранения у продукта не задан. */
function shelfLifeLabel(product: Product): string | undefined {
	return product.shelf_life_days === undefined
		? undefined
		: `хранится до ${shelfLifeDays(product.shelf_life_days)}`;
}

/** «Молочные продукты · хранится до 5 суток». */
export function productNote(product: Product): string {
	return [product.category, shelfLifeLabel(product)]
		.filter(Boolean)
		.join(' · ');
}
