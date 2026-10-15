import { createFileRoute } from '@tanstack/react-router';

import { menuQueryOptions } from '@entities/dish';
import { can } from '@entities/profile';
import { defaultHotDishesQueryOptions, OrderNewPage } from '@pages/orders';

import { requirePermission } from '../point-access';

export const Route = createFileRoute('/points/$pointId/orders_/new')({
	beforeLoad: ({ context }) =>
		requirePermission(context.point, 'orders.accept', 'Новый заказ'),
	// Меню и горячие предложения стартуют вместе с загрузкой кода страницы; ошибки покажет страница.
	loader: ({ context, params }) => {
		void context.queryClient.prefetchQuery(menuQueryOptions(params.pointId));

		if (can(context.point, 'recommendations.view')) {
			void context.queryClient.prefetchQuery(
				defaultHotDishesQueryOptions(params.pointId)
			);
		}
	},
	component: function OrderNewRoute() {
		return <OrderNewPage pointId={Route.useParams().pointId} />;
	},
});
