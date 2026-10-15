import { createFileRoute } from '@tanstack/react-router';

import { companyPointsQueryOptions } from '@entities/organization';
import { CompanyPage } from '@pages/company';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/company')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/company'),
	// Точки компании запрашиваются вместе с загрузкой кода страницы; ошибку покажет страница.
	loader: ({ context: { queryClient, point } }) => {
		void queryClient.prefetchQuery(companyPointsQueryOptions(point.company_id));
	},
	component: function CompanyRoute() {
		return <CompanyPage pointId={Route.useParams().pointId} />;
	},
});
