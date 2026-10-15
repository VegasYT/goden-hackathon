import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import { hotDishesQueryOptions } from '@entities/dish';
import {
	HotPage,
	hotSearchDefaults,
	hotSearchSchema,
	toHotDishParams,
} from '@pages/orders';

import { requirePermission } from '../point-access';

export const Route = createFileRoute('/points/$pointId/orders/hot')({
	validateSearch: hotSearchSchema,
	search: { middlewares: [stripSearchParams(hotSearchDefaults)] },
	loaderDeps: ({ search }) => toHotDishParams(search),
	beforeLoad: ({ context }) =>
		requirePermission(
			context.point,
			'recommendations.view',
			'Горячие предложения'
		),
	loader: ({ context, params, deps }) => {
		void context.queryClient.prefetchQuery(
			hotDishesQueryOptions(params.pointId, deps)
		);
	},
	component: function OrdersHotRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<HotPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
