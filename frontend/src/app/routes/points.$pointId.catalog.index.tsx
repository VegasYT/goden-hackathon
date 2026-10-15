import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import {
	productCategoriesQueryOptions,
	productListQueryOptions,
} from '@entities/product';
import {
	productCountQueryOptions,
	ProductsPage,
	productsSearchDefaults,
	productsSearchSchema,
	toProductParams,
} from '@pages/catalog';

export const Route = createFileRoute('/points/$pointId/catalog/')({
	validateSearch: productsSearchSchema,
	search: { middlewares: [stripSearchParams(productsSearchDefaults)] },
	loaderDeps: ({ search }) => search,
	// Запросы стартуют вместе с загрузкой кода страницы; ошибки покажет страница.
	loader: ({ context: { queryClient, point }, deps }) => {
		const companyId = point.company_id;

		void queryClient.prefetchQuery(
			productListQueryOptions(companyId, toProductParams(deps))
		);
		void queryClient.prefetchQuery(productCategoriesQueryOptions(companyId));
		void queryClient.prefetchQuery(productCountQueryOptions(companyId, true));
	},
	component: function CatalogProductsRoute() {
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<ProductsPage
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
