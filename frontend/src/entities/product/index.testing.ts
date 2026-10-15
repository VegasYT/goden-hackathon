import type { Product } from './api/product-queries';

/** Продукт каталога компании `demo` для тестов: граммы, без категории, срок 5 дней. */
export function testProduct(
	overrides: Pick<Product, 'id' | 'name'> & Partial<Product>
): Product {
	return {
		company_id: 'demo',
		archived: false,
		unit: 'g',
		category: '',
		shelf_life_days: 5,
		...overrides,
	};
}
