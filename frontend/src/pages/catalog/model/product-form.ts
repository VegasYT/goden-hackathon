import type { NewProduct, Product, ProductChange } from '@entities/product';
import type { QuantityUnit } from '@shared/lib/quantity';

/** Поля диалога продукта: срок — текстом поля, «Без срока» — флагом. */
export interface ProductDraft {
	name: string;
	unit: QuantityUnit;
	category: string;
	shelfLifeDays: string;
	noShelfLife: boolean;
}

export function productDraft(
	product: Product | undefined,
	initialName = ''
): ProductDraft {
	return {
		name: product?.name ?? initialName,
		unit: product?.unit ?? 'g',
		category: product?.category ?? '',
		shelfLifeDays: product?.shelf_life_days?.toString() ?? '',
		noShelfLife: product !== undefined && product.shelf_life_days === undefined,
	};
}

export function productProblems(
	draft: ProductDraft
): Partial<Record<'name' | 'shelfLifeDays', string>> {
	const problems: Partial<Record<'name' | 'shelfLifeDays', string>> = {};

	if (!draft.name.trim()) {
		problems.name = 'Введите название';
	}

	if (!draft.noShelfLife && !(Number(draft.shelfLifeDays) >= 1)) {
		problems.shelfLifeDays = 'Укажите срок или отметьте «Без срока»';
	}

	return problems;
}

const shelfLife = (draft: ProductDraft) =>
	draft.noShelfLife ? undefined : Number(draft.shelfLifeDays);

export function toNewProduct(draft: ProductDraft): NewProduct {
	const days = shelfLife(draft);

	return {
		name: draft.name.trim(),
		unit: draft.unit,
		...(draft.category.trim() ? { category: draft.category.trim() } : {}),
		...(days === undefined ? {} : { shelf_life_days: days }),
	};
}

/** Только изменённые поля; снятый срок отправляется как `null`. */
export function toProductChange(
	product: Product,
	draft: ProductDraft
): ProductChange {
	const change: ProductChange = {};
	const name = draft.name.trim();
	const category = draft.category.trim();
	const days = shelfLife(draft) ?? null;

	if (name !== product.name) {
		change.name = name;
	}

	if (draft.unit !== product.unit) {
		change.unit = draft.unit;
	}

	if (category !== product.category) {
		change.category = category;
	}

	if (days !== (product.shelf_life_days ?? null)) {
		change.shelf_life_days = days;
	}

	return change;
}
