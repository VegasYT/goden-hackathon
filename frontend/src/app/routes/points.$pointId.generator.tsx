import { createFileRoute } from '@tanstack/react-router';

import { deliveryListQueryOptions } from '@entities/delivery';
import { catalogProductsQueryOptions } from '@entities/product';
import {
	GeneratorPage,
	generatorQueryOptions,
	waitingTestDeliveries,
} from '@pages/generator';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/generator')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/generator'),
	// Каталог и счётчик стартуют вместе с настройками; ошибки этих запросов покажет страница.
	loader: ({ context: { queryClient, point }, params: { pointId } }) => {
		void queryClient.prefetchQuery(
			catalogProductsQueryOptions(point.company_id)
		);
		void queryClient.prefetchQuery(
			deliveryListQueryOptions(pointId, waitingTestDeliveries)
		);

		return queryClient.ensureQueryData(generatorQueryOptions(pointId));
	},
	component: function GeneratorRoute() {
		return <GeneratorPage pointId={Route.useParams().pointId} />;
	},
});
