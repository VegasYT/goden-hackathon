import { createFileRoute, redirect } from '@tanstack/react-router';

import { meQueryOptions, readLastPoint, startPoint } from '@entities/profile';

export const Route = createFileRoute('/')({
	beforeLoad: async ({ context }) => {
		const profile = await context.queryClient.ensureQueryData(meQueryOptions);
		const point = startPoint(profile, readLastPoint(context.session.userId));

		if (!point) {
			throw redirect({ to: '/welcome' });
		}

		throw redirect({ to: '/points/$pointId', params: { pointId: point.id } });
	},
});
