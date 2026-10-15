import { createFileRoute } from '@tanstack/react-router';

import { menuQueryOptions } from '@entities/dish';
import { StopListPage } from '@pages/stop-list';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/stop-list')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/stop-list'),
	// Запрос стартует вместе с загрузкой кода страницы; ошибку покажет страница.
	loader: ({ context, params }) => {
		void context.queryClient.prefetchQuery(menuQueryOptions(params.pointId));
	},
	component: function StopListRoute() {
		const { pointId } = Route.useParams();

		return <StopListPage pointId={pointId} />;
	},
});
