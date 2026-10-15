import type { QueryClient } from '@tanstack/react-query';

import { organizationKeys, type Point } from '../api/organization';

/** Кладёт точку из ответа записи в кэш: саму точку и её строку в точках компании, новую — в конец. */
export function storePoint(queryClient: QueryClient, point: Point) {
	queryClient.setQueryData(organizationKeys.point(point.id), point);
	queryClient.setQueryData<Point[]>(
		organizationKeys.companyPoints(point.company_id),
		(points) =>
			points &&
			(points.some(({ id }) => id === point.id)
				? points.map((item) => (item.id === point.id ? point : item))
				: [...points, point])
	);
}
