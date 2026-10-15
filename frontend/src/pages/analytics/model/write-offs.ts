import type { Product } from '@entities/product';

import type { WriteOffStatistic } from '../api/analytics';

/** Количество продукта в его единице. */
interface ProductQuantity {
	product: Product;
	quantity: number;
}

export interface ProductWriteOffs extends ProductQuantity {
	/** Сумма по всем причинам. */
	operations: number;
	reasons: string[];
}

export interface ReasonWriteOffs {
	reason: string;
	operations: number;
	/** Продукты причины в том же порядке, что и список по продуктам. */
	products: ProductQuantity[];
}

// Количества в разных единицах несравнимы, поэтому порядок - внутри единицы, как у сервера.
function byUnitAndQuantity(a: ProductQuantity, b: ProductQuantity): number {
	return (
		a.product.unit.localeCompare(b.product.unit) ||
		b.quantity - a.quantity ||
		a.product.name.localeCompare(b.product.name, 'ru')
	);
}

/** Списания по продуктам: сумма по всем причинам. */
export function writeOffsByProduct(
	items: WriteOffStatistic[]
): ProductWriteOffs[] {
	const products = new Map<string, ProductWriteOffs>();

	for (const item of items) {
		const entry = products.get(item.product.id) ?? {
			product: item.product,
			quantity: 0,
			operations: 0,
			reasons: [],
		};

		entry.quantity += item.quantity;
		entry.operations += item.operations;
		entry.reasons.push(item.reason);
		products.set(item.product.id, entry);
	}

	return [...products.values()].sort(byUnitAndQuantity);
}

/** Списания по причинам: число операций и продукты, от частой причины к редкой. */
export function writeOffsByReason(
	items: WriteOffStatistic[]
): ReasonWriteOffs[] {
	const reasons = new Map<string, ReasonWriteOffs>();

	for (const item of items) {
		const entry = reasons.get(item.reason) ?? {
			reason: item.reason,
			operations: 0,
			products: [],
		};

		entry.operations += item.operations;
		entry.products.push({ product: item.product, quantity: item.quantity });
		reasons.set(item.reason, entry);
	}

	for (const entry of reasons.values()) {
		entry.products.sort(byUnitAndQuantity);
	}

	return [...reasons.values()].sort((a, b) => b.operations - a.operations);
}
