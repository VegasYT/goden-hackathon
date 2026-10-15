import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import {
	ReorderPage,
	deliveryRecommendationsQueryOptions,
	reorderSearchDefaults,
	reorderSearchSchema,
	toRecommendationPage,
} from '@pages/stock';

import { requirePermission } from '../point-access';

export const Route = createFileRoute('/points/$pointId/stock/reorder')({
	validateSearch: reorderSearchSchema,
	search: { middlewares: [stripSearchParams(reorderSearchDefaults)] },
	loaderDeps: ({ search }) => ({ ...search, history: undefined }),
	beforeLoad: ({ context }) =>
		requirePermission(context.point, 'recommendations.view', 'Пора заказать'),
	loader: ({ context, params, deps }) => {
		void context.queryClient.prefetchQuery(
			deliveryRecommendationsQueryOptions(
				params.pointId,
				toRecommendationPage(deps)
			)
		);
	},
	component: function StockReorderRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<ReorderPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
