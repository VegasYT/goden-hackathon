import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import { menuQueryOptions, pointDishesQueryOptions } from '@entities/dish';
import {
	KitchenPage,
	kitchenSearchDefaults,
	kitchenSearchSchema,
} from '@pages/kitchen';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/kitchen')({
	validateSearch: kitchenSearchSchema,
	search: { middlewares: [stripSearchParams(kitchenSearchDefaults)] },
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/kitchen'),
	// Запросы стартуют вместе с загрузкой кода страницы, а не после неё; ошибки покажет страница.
	loader: ({ context, params }) => {
		void context.queryClient.prefetchQuery(menuQueryOptions(params.pointId));
		void context.queryClient.prefetchQuery(
			pointDishesQueryOptions(params.pointId)
		);
	},
	component: function KitchenRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<KitchenPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
