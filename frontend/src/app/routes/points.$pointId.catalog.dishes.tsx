import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import { companyDishesQueryOptions } from '@entities/dish';
import {
	dishCountQueryOptions,
	DishesPage,
	dishesSearchDefaults,
	dishesSearchSchema,
	toDishParams,
} from '@pages/catalog';

export const Route = createFileRoute('/points/$pointId/catalog/dishes')({
	validateSearch: dishesSearchSchema,
	search: { middlewares: [stripSearchParams(dishesSearchDefaults)] },
	// Открытая панель не меняет список, поэтому в зависимости загрузки не входит.
	loaderDeps: ({ search }) => ({ ...search, dish: undefined }),
	loader: ({ context: { queryClient, point }, deps }) => {
		const companyId = point.company_id;

		void queryClient.prefetchQuery(
			companyDishesQueryOptions(companyId, toDishParams(deps))
		);
		void queryClient.prefetchQuery(dishCountQueryOptions(companyId, 'missing'));
		void queryClient.prefetchQuery(
			dishCountQueryOptions(companyId, 'archived')
		);
	},
	component: function CatalogDishesRoute() {
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<DishesPage
				search={search}
				onSearchChange={(patch, options) =>
					void navigate({
						search: (prev) => ({ ...prev, ...patch }),
						replace: options?.replace,
					})
				}
			/>
		);
	},
});
