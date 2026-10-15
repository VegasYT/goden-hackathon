import { createFileRoute, stripSearchParams } from '@tanstack/react-router';

import {
	AnalyticsPage,
	analyticsSearchDefaults,
	analyticsSearchSchema,
	prefetchAnalytics,
} from '@pages/analytics';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/analytics')({
	validateSearch: analyticsSearchSchema,
	search: { middlewares: [stripSearchParams(analyticsSearchDefaults)] },
	// Вид списаний и продукт графика запросы не меняют, поэтому в зависимости загрузки не входят.
	loaderDeps: ({ search: { horizon, period, from, to } }) => ({
		horizon,
		period,
		from,
		to,
	}),
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/analytics'),
	// Запросы стартуют вместе с загрузкой кода страницы; ошибки покажет страница.
	loader: ({ context, params, deps }) =>
		prefetchAnalytics(context.queryClient, params.pointId, deps),
	component: function AnalyticsRoute() {
		const { pointId } = Route.useParams();
		const search = Route.useSearch();
		const navigate = Route.useNavigate();

		return (
			<AnalyticsPage
				pointId={pointId}
				search={search}
				onSearchChange={(patch) =>
					void navigate({ search: (prev) => ({ ...prev, ...patch }) })
				}
			/>
		);
	},
});
