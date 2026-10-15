import { createFileRoute } from '@tanstack/react-router';

import { pointQueryOptions } from '@entities/organization';
import { PointSettingsPage } from '@pages/point-settings';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/settings')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/settings'),
	// Запрос стартует вместе с загрузкой кода страницы; ошибку покажет страница.
	loader: ({ context, params }) => {
		void context.queryClient.prefetchQuery(pointQueryOptions(params.pointId));
	},
	component: function PointSettingsRoute() {
		return <PointSettingsPage pointId={Route.useParams().pointId} />;
	},
});
