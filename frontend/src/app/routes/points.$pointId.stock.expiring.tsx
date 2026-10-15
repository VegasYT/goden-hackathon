import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import { expiringBatchesQueryOptions } from '@entities/stock';
import {
	ExpiringPage,
	expiringSearchDefaults,
	expiringSearchSchema,
	stockSummaryQueryOptions,
	toExpiringParams,
} from '@pages/stock';

export const Route = createFileRoute('/points/$pointId/stock/expiring')({
	validateSearch: expiringSearchSchema,
	search: { middlewares: [stripSearchParams(expiringSearchDefaults)] },
	loaderDeps: ({ search }) => ({ ...search, history: undefined }),
	loader: ({ context, params, deps }) => {
		void context.queryClient.prefetchQuery(
			expiringBatchesQueryOptions(params.pointId, toExpiringParams(deps))
		);
		// Плашку просрочки считает сводка склада: её запрос стартует вместе со списком.
		void context.queryClient.prefetchQuery(
			stockSummaryQueryOptions(params.pointId, {})
		);
	},
	component: function StockExpiringRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<ExpiringPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
