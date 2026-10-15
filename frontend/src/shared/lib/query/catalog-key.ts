/** Префикс ключей каталога компании: продукты, блюда и категории сбрасываются им одним действием. */
export const catalogKey = (companyId: string) =>
	['companies', companyId, 'catalog'] as const;
