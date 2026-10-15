import {
	createFileRoute,
	Outlet,
	useChildMatches,
} from '@tanstack/react-router';

import { can } from '@entities/profile';
import {
	defaultHotDishesQueryOptions,
	OrdersLayout,
	type OrdersTab,
} from '@pages/orders';

import { requireSection } from '../point-access';

const tabs: Record<string, OrdersTab> = {
	'/points/$pointId/orders/hot': 'hot',
};

export const Route = createFileRoute('/points/$pointId/orders')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/orders'),
	// Число на вкладке предложений запрашивается вместе с загрузкой кода страницы.
	loader: ({ context, params }) => {
		if (can(context.point, 'recommendations.view')) {
			void context.queryClient.prefetchQuery(
				defaultHotDishesQueryOptions(params.pointId)
			);
		}
	},
	component: function OrdersRoute() {
		const { pointId } = Route.useParams();
		const child = useChildMatches({ select: (matches) => matches[0]?.routeId });

		return (
			<OrdersLayout pointId={pointId} tab={(child && tabs[child]) ?? 'history'}>
				<Outlet />
			</OrdersLayout>
		);
	},
});
