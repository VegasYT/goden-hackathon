import { createContext, useContext } from 'react';

/** Каталог открытой точки: компания, можно ли его менять и диалоги создания из любого места раздела. */
export interface CatalogScope {
	pointId: string;
	companyId: string;
	/** Компания отключена: каталог только для чтения. */
	readOnly: boolean;
	newProduct: () => void;
	newDish: () => void;
}

export const CatalogScopeContext = createContext<CatalogScope | null>(null);

export function useCatalogScope(): CatalogScope {
	const scope = useContext(CatalogScopeContext);

	if (!scope) {
		throw new Error('useCatalogScope вызван вне CatalogLayout');
	}

	return scope;
}
