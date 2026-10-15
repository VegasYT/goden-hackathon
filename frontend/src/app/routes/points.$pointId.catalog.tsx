import {
	createFileRoute,
	Outlet,
	useChildMatches,
} from '@tanstack/react-router';

import {
	CatalogLayout,
	dishCountQueryOptions,
	productCountQueryOptions,
} from '@pages/catalog';

import { requireSection } from '../point-access';

export const Route = createFileRoute('/points/$pointId/catalog')({
	beforeLoad: ({ context }) =>
		requireSection(context.point, '/points/$pointId/catalog'),
	// Числа на вкладках запрашиваются вместе с загрузкой кода страницы.
	loader: ({ context: { queryClient, point } }) => {
		void queryClient.prefetchQuery(
			productCountQueryOptions(point.company_id, false)
		);
		void queryClient.prefetchQuery(
			dishCountQueryOptions(point.company_id, 'active')
		);
	},
	component: function CatalogRoute() {
		const { pointId } = Route.useParams();
		const child = useChildMatches({ select: (matches) => matches[0]?.routeId });

		return (
			<CatalogLayout
				pointId={pointId}
				tab={
					child === '/points/$pointId/catalog/dishes' ? 'dishes' : 'products'
				}
			>
				<Outlet />
			</CatalogLayout>
		);
	},
});
