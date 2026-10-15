import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import { productCategoriesQueryOptions } from '@entities/product';
import {
	StockPage,
	stockListQueryOptions,
	stockSearchDefaults,
	stockSearchSchema,
	stockSummaryQueryOptions,
	toFilterParams,
	toStockParams,
} from '@pages/stock';

export const Route = createFileRoute('/points/$pointId/stock/')({
	validateSearch: stockSearchSchema,
	search: { middlewares: [stripSearchParams(stockSearchDefaults)] },
	// Открытая история не меняет запросы склада, поэтому в зависимости загрузки не входит.
	loaderDeps: ({ search }) => ({ ...search, history: undefined }),
	// Запросы стартуют вместе с загрузкой кода страницы; ошибки покажет страница.
	loader: ({ context, params, deps }) => {
		void context.queryClient.prefetchQuery(
			stockListQueryOptions(params.pointId, toStockParams(deps))
		);
		void context.queryClient.prefetchQuery(
			stockSummaryQueryOptions(params.pointId, toFilterParams(deps))
		);
		void context.queryClient.prefetchQuery(
			productCategoriesQueryOptions(context.point.company_id)
		);
	},
	component: function StockLevelsRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<StockPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
