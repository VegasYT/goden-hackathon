import {
	createFileRoute,
	Outlet,
	useChildMatches,
} from '@tanstack/react-router';

import { can } from '@entities/profile';
import {
	reorderCountQueryOptions,
	StockLayout,
	stockLayoutSearchSchema,
	urgentExpiringQueryOptions,
	type StockTab,
} from '@pages/stock';

import { requireSection } from '../point-access';

const tabs: Record<string, StockTab> = {
	'/points/$pointId/stock/expiring': 'expiring',
	'/points/$pointId/stock/reorder': 'reorder',
};

export const Route = createFileRoute('/points/$pointId/stock')({
	validateSearch: stockLayoutSearchSchema,
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/stock'),
	// Числа на вкладках запрашиваются вместе с загрузкой кода страницы.
	loader: ({ context, params }) => {
		void context.queryClient.prefetchQuery(
			urgentExpiringQueryOptions(params.pointId)
		);

		if (can(context.point, 'recommendations.view')) {
			void context.queryClient.prefetchQuery(
				reorderCountQueryOptions(params.pointId)
			);
		}
	},
	component: function StockRoute() {
		const { pointId } = Route.useParams();
		const { history } = Route.useSearch();
		const child = useChildMatches({ select: (matches) => matches[0]?.routeId });

		return (
			<StockLayout
				pointId={pointId}
				tab={(child && tabs[child]) ?? 'levels'}
				history={history}
			>
				<Outlet />
			</StockLayout>
		);
	},
});
