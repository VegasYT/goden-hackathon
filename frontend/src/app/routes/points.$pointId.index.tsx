import { createFileRoute } from '@tanstack/react-router';

import { DashboardPage, prefetchDashboard } from '@pages/dashboard';

export const Route = createFileRoute('/points/$pointId/')({
	loader: ({ context }) => {
		prefetchDashboard(context.queryClient, context.point);
	},
	component: function DashboardRoute() {
		return <DashboardPage pointId={Route.useParams().pointId} />;
	},
});
