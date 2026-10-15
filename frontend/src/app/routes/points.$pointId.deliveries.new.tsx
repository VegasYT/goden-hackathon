import { createFileRoute } from '@tanstack/react-router';

import { productListQueryOptions } from '@entities/product';
import { DeliveryNewPage } from '@pages/delivery-new';

import { requirePermission } from '../point-access';

export const Route = createFileRoute('/points/$pointId/deliveries/new')({
	beforeLoad: ({ context }) =>
		requirePermission(context.point, 'deliveries.register', 'Новая поставка'),
	// Каталог стартует вместе с загрузкой кода страницы; ошибку покажет страница.
	loader: ({ context }) => {
		void context.queryClient.prefetchQuery(
			productListQueryOptions(context.point.company_id, { limit: 1, offset: 0 })
		);
	},
	component: function DeliveryNewRoute() {
		return <DeliveryNewPage pointId={Route.useParams().pointId} />;
	},
});
