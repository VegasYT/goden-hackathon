import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import {
	HistoryPage,
	historySearchDefaults,
	historySearchSchema,
	orderListQueryOptions,
	toOrderListParams,
} from '@pages/orders';

export const Route = createFileRoute('/points/$pointId/orders/')({
	validateSearch: historySearchSchema,
	search: { middlewares: [stripSearchParams(historySearchDefaults)] },
	loaderDeps: ({ search }) => toOrderListParams(search),
	// Запрос стартует вместе с загрузкой кода страницы; ошибку покажет страница.
	loader: ({ context, params, deps }) => {
		void context.queryClient.prefetchQuery(
			orderListQueryOptions(params.pointId, deps)
		);
	},
	component: function OrdersHistoryRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<HistoryPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
