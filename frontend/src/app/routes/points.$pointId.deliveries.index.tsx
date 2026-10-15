import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import {
	deliveryListQueryOptions,
	deliverySummaryQueryOptions,
} from '@entities/delivery';
import {
	DeliveriesPage,
	deliveryListDefaults,
	deliveryListSearchSchema,
	openQuarantineQueryOptions,
	toListParams,
} from '@pages/deliveries';

export const Route = createFileRoute('/points/$pointId/deliveries/')({
	validateSearch: deliveryListSearchSchema,
	search: { middlewares: [stripSearchParams(deliveryListDefaults)] },
	// Открытая панель не меняет запросы страницы, поэтому в зависимости загрузки не входит.
	loaderDeps: ({ search }) => toListParams(search),
	// Ссылка с открытой панелью карантина сразу запрашивает и её позиции.
	beforeLoad: ({ context, params, search }) => {
		if (search.panel === 'quarantine') {
			void context.queryClient.prefetchQuery(
				openQuarantineQueryOptions(params.pointId)
			);
		}
	},
	// Запросы стартуют вместе с загрузкой кода страницы, а не после неё; ошибки покажет страница.
	loader: ({ context, params, deps }) => {
		void context.queryClient.prefetchQuery(
			deliveryListQueryOptions(params.pointId, deps)
		);
		void context.queryClient.prefetchQuery(
			deliverySummaryQueryOptions(params.pointId)
		);
	},
	component: function DeliveriesRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<DeliveriesPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
