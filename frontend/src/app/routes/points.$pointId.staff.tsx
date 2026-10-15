import { createFileRoute } from '@tanstack/react-router';

import { staffQueryOptions } from '@entities/organization';
import { StaffPage } from '@pages/staff';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/staff')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/staff'),
	// Запрос стартует вместе с загрузкой кода страницы; ошибку покажет страница.
	loader: ({ context, params }) => {
		void context.queryClient.prefetchQuery(staffQueryOptions(params.pointId));
	},
	component: function StaffRoute() {
		return <StaffPage pointId={Route.useParams().pointId} />;
	},
});
